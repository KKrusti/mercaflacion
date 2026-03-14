// Command seed populates the database by importing Mercadona PDF receipts.
//
// Usage:
//
//	go run ./cmd/seed [flags] <pdf-file> [<pdf-file> ...]
//
// Flags:
//
//	-dir string     directory containing PDF files to import
//	-workers int    number of parallel PDF workers (default: number of CPU cores)
//
// Requires DATABASE_URL environment variable.
package main

import (
	"basket-cost/pkg/database"
	"basket-cost/pkg/store"
	"basket-cost/pkg/ticket"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

type fileResult struct {
	path      string
	result    *ticket.ImportResult
	importErr error
}

func main() {
	dirPath := flag.String("dir", "", "directory of PDF files to import")
	workers := flag.Int("workers", runtime.NumCPU(), "number of parallel PDF workers")
	flag.Parse()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL is not set")
	}

	db, err := database.OpenDSN(dsn)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer db.Close()

	s := store.New(db)
	newImp := func() *ticket.Importer {
		return ticket.NewImporter(ticket.NewExtractor(), ticket.NewMercadonaParser(), s)
	}

	var paths []string
	if *dirPath != "" {
		entries, err := os.ReadDir(*dirPath)
		if err != nil {
			log.Fatalf("read dir %q: %v", *dirPath, err)
		}
		for _, e := range entries {
			if e.Type()&fs.ModeSymlink != 0 {
				continue
			}
			if !e.IsDir() && strings.EqualFold(filepath.Ext(e.Name()), ".pdf") {
				paths = append(paths, filepath.Join(*dirPath, e.Name()))
			}
		}
	}
	paths = append(paths, flag.Args()...)

	if len(paths) == 0 {
		flag.Usage()
		os.Exit(1)
	}

	numWorkers := *workers
	if numWorkers < 1 {
		numWorkers = 1
	}

	jobs := make(chan string, len(paths))
	results := make(chan fileResult, len(paths))

	var wg sync.WaitGroup
	for range numWorkers {
		wg.Add(1)
		imp := newImp()
		go func() {
			defer wg.Done()
			for p := range jobs {
				r, err := importFile(imp, p)
				results <- fileResult{path: p, result: r, importErr: err}
			}
		}()
	}

	for _, p := range paths {
		jobs <- p
	}
	close(jobs)

	go func() {
		wg.Wait()
		close(results)
	}()

	var totalImported int
	var totalErrors int

	for res := range results {
		if res.result != nil {
			totalImported += res.result.LinesImported
			fmt.Printf("OK  %-50s  invoice=%s  lines=%d\n",
				filepath.Base(res.path), res.result.InvoiceNumber, res.result.LinesImported)
		}
		if res.importErr != nil {
			totalErrors++
			fmt.Fprintf(os.Stderr, "ERR %-50s  %v\n", filepath.Base(res.path), res.importErr)
		}
	}

	fmt.Printf("\n--- Resultado ---\n")
	fmt.Printf("Archivos procesados : %d\n", len(paths))
	fmt.Printf("Workers             : %d\n", numWorkers)
	fmt.Printf("Líneas importadas   : %d\n", totalImported)
	fmt.Printf("Errores             : %d\n", totalErrors)

	if totalErrors > 0 {
		os.Exit(1)
	}
}

func importFile(imp *ticket.Importer, path string) (*ticket.ImportResult, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat file: %w", err)
	}
	return imp.Import(0, f, info.Size())
}
