// Command dbsanitize detects and optionally removes duplicate price records.
//
// A duplicate is defined as two or more rows in price_records that share the
// same (product_id, date, price, store, user_id). This can happen when the
// same PDF ticket was uploaded more than once (e.g. under a different filename
// or before the processed_files guard was in place).
//
// The tool reports every duplicate group, then waits for the user to type
// "yes" before deleting. The earliest record (lowest id) in each group is
// kept; the rest are removed. A VACUUM is run afterwards to reclaim space.
//
// Usage:
//
//	go run ./cmd/dbsanitize -db basket-cost.db
package main

import (
	"bufio"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	_ "modernc.org/sqlite"
)

type duplicateGroup struct {
	productID string
	date      string
	price     float64
	store     string
	userID    sql.NullInt64
	count     int   // total rows in group (including the one to keep)
	keepID    int64 // MIN(id) — the row that will be kept
}

func main() {
	dbPath := flag.String("db", "basket-cost.db", "path to the SQLite database file")
	flag.Parse()

	db, err := sql.Open("sqlite", *dbPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		log.Fatalf("apply pragmas: %v", err)
	}

	groups, err := findDuplicates(db)
	if err != nil {
		log.Fatalf("find duplicates: %v", err)
	}

	if len(groups) == 0 {
		fmt.Println("No duplicate price records found. Database is clean.")
		return
	}

	printReport(groups)

	totalDupes := 0
	for _, g := range groups {
		totalDupes += g.count - 1
	}
	fmt.Printf("\n%d duplicate row(s) will be deleted (keeping the earliest record per group).\n", totalDupes)
	fmt.Print("\nType 'yes' to confirm deletion, anything else to cancel: ")

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	if strings.TrimSpace(scanner.Text()) != "yes" {
		fmt.Println("Cancelled. No changes made.")
		return
	}

	deleted, err := deleteDuplicates(db)
	if err != nil {
		log.Fatalf("delete duplicates: %v", err)
	}

	fmt.Printf("Deleted %d duplicate row(s).\n", deleted)

	if _, err := db.Exec("VACUUM"); err != nil {
		log.Printf("warning: vacuum failed: %v", err)
	} else {
		fmt.Println("Database compacted (VACUUM).")
	}
}

func findDuplicates(db *sql.DB) ([]duplicateGroup, error) {
	rows, err := db.Query(`
		SELECT
			product_id,
			date,
			price,
			store,
			user_id,
			COUNT(*)  AS cnt,
			MIN(id)   AS keep_id
		FROM price_records
		GROUP BY product_id, date, price, store, user_id
		HAVING cnt > 1
		ORDER BY cnt DESC, product_id, date
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groups []duplicateGroup
	for rows.Next() {
		var g duplicateGroup
		if err := rows.Scan(&g.productID, &g.date, &g.price, &g.store, &g.userID, &g.count, &g.keepID); err != nil {
			return nil, err
		}
		groups = append(groups, g)
	}
	return groups, rows.Err()
}

func printReport(groups []duplicateGroup) {
	const colFmt = "%-42s  %-10s  %8s  %-14s  %8s  %6s\n"
	fmt.Printf("Found %d group(s) of duplicate price records:\n\n", len(groups))
	fmt.Printf(colFmt, "PRODUCT", "DATE", "PRICE", "STORE", "USER_ID", "DUPES")
	fmt.Println(strings.Repeat("-", 100))

	for _, g := range groups {
		userStr := "NULL"
		if g.userID.Valid {
			userStr = fmt.Sprintf("%d", g.userID.Int64)
		}
		fmt.Printf(colFmt,
			truncate(g.productID, 42),
			g.date,
			fmt.Sprintf("%.2f €", g.price),
			truncate(g.store, 14),
			userStr,
			fmt.Sprintf("×%d", g.count-1),
		)
	}
}

func deleteDuplicates(db *sql.DB) (int64, error) {
	result, err := db.Exec(`
		DELETE FROM price_records
		WHERE id NOT IN (
			SELECT MIN(id)
			FROM price_records
			GROUP BY product_id, date, price, store, user_id
		)
	`)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// truncate clips s to max runes, appending "…" if it was cut.
func truncate(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max-1]) + "…"
}
