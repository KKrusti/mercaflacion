// Package emailfetcher polls IMAP mailboxes and extracts PDF attachments from
// Mercadona digital receipts, then feeds them through the ticket import pipeline.
package emailfetcher

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
	"github.com/emersion/go-message/mail"

	"basket-cost/pkg/crypto"
	"basket-cost/pkg/models"
	"basket-cost/pkg/ticket"
)

// AccountStore is the subset of store.Store required by the Fetcher.
type AccountStore interface {
	GetAllEmailAccounts() ([]models.EmailAccount, error)
	UpdateEmailAccountLastUID(id int64, uid uint32) error
}

// Fetcher polls IMAP accounts and imports new PDF attachments.
type Fetcher struct {
	accounts  AccountStore
	importer  *ticket.Importer
	cryptoKey []byte
}

// New returns a Fetcher ready to poll.
// cryptoKey must be the same 32-byte key used when the password was encrypted.
func New(accounts AccountStore, importer *ticket.Importer, cryptoKey []byte) *Fetcher {
	return &Fetcher{
		accounts:  accounts,
		importer:  importer,
		cryptoKey: cryptoKey,
	}
}

// PollAll fetches new PDF attachments from every registered account and imports them.
// Errors for individual accounts are logged and do not abort the remaining accounts.
func (f *Fetcher) PollAll(ctx context.Context) {
	accounts, err := f.accounts.GetAllEmailAccounts()
	if err != nil {
		log.Printf("emailfetcher: list accounts: %v", err)
		return
	}
	for _, acc := range accounts {
		select {
		case <-ctx.Done():
			return
		default:
		}
		if err := f.pollAccount(ctx, acc); err != nil {
			log.Printf("emailfetcher: account %s (user %d): %v", acc.EmailAddress, acc.UserID, err)
		}
	}
}

// pollAccount connects to the IMAP server, fetches messages with UIDs greater
// than last_uid_seen, extracts PDF attachments, and imports them.
func (f *Fetcher) pollAccount(ctx context.Context, acc models.EmailAccount) error {
	password, err := crypto.Decrypt(acc.EncryptedPassword, f.cryptoKey)
	if err != nil {
		return fmt.Errorf("decrypt password: %w", err)
	}

	addr := fmt.Sprintf("%s:%d", acc.IMAPHost, acc.IMAPPort)
	opts := &imapclient.Options{
		TLSConfig: &tls.Config{ServerName: acc.IMAPHost},
	}
	c, err := imapclient.DialTLS(addr, opts)
	if err != nil {
		return fmt.Errorf("dial %s: %w", addr, err)
	}
	defer c.Logout() //nolint:errcheck

	if err := c.Login(acc.EmailAddress, password).Wait(); err != nil {
		return fmt.Errorf("login: %w", err)
	}

	if _, err := c.Select("INBOX", nil).Wait(); err != nil {
		return fmt.Errorf("select INBOX: %w", err)
	}

	// Build UID range: last_uid_seen+1 to * (last message).
	// If last_uid_seen is 0 we fetch everything.
	var seqSet imap.UIDSet
	seqSet.AddRange(imap.UID(acc.LastUIDSeen+1), 0) // 0 = "*"

	section := &imap.FetchItemBodySection{}
	fetchOpts := &imap.FetchOptions{
		UID:         true,
		BodySection: []*imap.FetchItemBodySection{section},
	}

	messages, err := c.Fetch(seqSet, fetchOpts).Collect()
	if err != nil {
		return fmt.Errorf("fetch messages: %w", err)
	}
	if len(messages) == 0 {
		return nil
	}

	var maxUID imap.UID
	for _, msg := range messages {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if msg.UID > maxUID {
			maxUID = msg.UID
		}

		raw := msg.FindBodySection(section)
		if len(raw) == 0 {
			continue
		}

		pdfs, err := extractPDFs(raw)
		if err != nil {
			log.Printf("emailfetcher: extract PDFs from UID %d: %v", msg.UID, err)
			continue
		}
		for _, pdf := range pdfs {
			result, err := f.importer.Import(acc.UserID, bytes.NewReader(pdf), int64(len(pdf)))
			if err != nil {
				log.Printf("emailfetcher: import PDF from UID %d: %v", msg.UID, err)
				continue
			}
			log.Printf("emailfetcher: user %d — imported ticket %s (%d lines) from UID %d",
				acc.UserID, result.InvoiceNumber, result.LinesImported, msg.UID)
		}
	}

	if maxUID > 0 {
		if err := f.accounts.UpdateEmailAccountLastUID(acc.ID, uint32(maxUID)); err != nil {
			log.Printf("emailfetcher: update last_uid_seen for account %d: %v", acc.ID, err)
		}
	}
	return nil
}

// extractPDFs walks the MIME tree of the raw message bytes and returns the raw
// bytes of every PDF attachment found.
func extractPDFs(raw []byte) ([][]byte, error) {
	mr, err := mail.CreateReader(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("parse mail: %w", err)
	}

	var pdfs [][]byte
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("emailfetcher: skip mail part: %v", err)
			continue
		}

		att, ok := part.Header.(*mail.AttachmentHeader)
		if !ok {
			continue
		}

		filename, _ := att.Filename()
		ct, _, _ := att.ContentType()

		if !isPDF(filename, ct) {
			continue
		}

		data, err := io.ReadAll(part.Body)
		if err != nil {
			log.Printf("emailfetcher: read attachment %q: %v", filename, err)
			continue
		}
		pdfs = append(pdfs, data)
	}
	return pdfs, nil
}

// isPDF returns true when the filename has a .pdf extension or the MIME type
// indicates a PDF, as a defence against mis-labelled attachments.
func isPDF(filename, contentType string) bool {
	if strings.HasSuffix(strings.ToLower(filename), ".pdf") {
		return true
	}
	return strings.EqualFold(contentType, "application/pdf")
}

// RunPoller blocks, calling PollAll on interval until ctx is cancelled.
func RunPoller(ctx context.Context, f *Fetcher, interval time.Duration) {
	// Poll immediately on startup so fresh deployments pick up pending emails.
	log.Printf("emailfetcher: initial poll (interval=%v)", interval)
	f.PollAll(ctx)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			log.Printf("emailfetcher: scheduled poll")
			f.PollAll(ctx)
		}
	}
}
