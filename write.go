package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/maxmind/mmdbwriter"
	"github.com/maxmind/mmdbwriter/mmdbtype"
	maxminddb "github.com/oschwald/maxminddb-golang/v2"
	"go4.org/netipx"
)

func writeMMDB(entries map[string]*Entry, output Output) error {
	if err := output.validate(); err != nil {
		return err
	}
	writer, err := mmdbwriter.New(mmdbwriter.Options{
		DatabaseType:            "GeoLite2-Country",
		Description:             map[string]string{"en": "GeoIP-CN Country database"},
		RecordSize:              28,
		IncludeReservedNetworks: true,
	})
	if err != nil {
		return fmt.Errorf("create mmdb writer: %w", err)
	}

	for _, name := range output.WantedList {
		name = strings.ToUpper(strings.TrimSpace(name))
		entry, ok := entries[name]
		if !ok || entry == nil {
			return fmt.Errorf("requested entry %s not found", name)
		}

		record := mmdbtype.Map{
			"country": mmdbtype.Map{
				"iso_code": mmdbtype.String(name),
			},
		}

		prefixes, err := entry.Prefixes()
		if err != nil {
			return fmt.Errorf("get prefixes for %s: %w", name, err)
		}
		if len(prefixes) == 0 {
			return fmt.Errorf("requested entry %s contains no IP ranges", name)
		}

		for _, prefix := range prefixes {
			if err := writer.Insert(netipx.PrefixIPNet(prefix), record); err != nil {
				return fmt.Errorf("insert %s: %w", prefix, err)
			}
		}

		log.Printf("added %s: %d prefixes", name, len(prefixes))
	}

	if err := os.MkdirAll(output.Dir, 0755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	outPath := filepath.Join(output.Dir, output.File)
	if err := writeMMDBFile(outPath, func(w io.Writer) error {
		_, err := writer.WriteTo(w)
		return err
	}); err != nil {
		return err
	}
	log.Printf("wrote %s", outPath)
	return nil
}

// Validate and close a temporary file before replacing the previous database.
// Keeping both files in the same directory makes rename atomic on Unix.
func writeMMDBFile(outPath string, write func(io.Writer) error) error {
	previous, err := os.Stat(outPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("stat previous output: %w", err)
	}
	f, err := os.CreateTemp(filepath.Dir(outPath), "."+filepath.Base(outPath)+"-*")
	if err != nil {
		return fmt.Errorf("create temporary output: %w", err)
	}
	defer func() {
		f.Close()
		os.Remove(f.Name())
	}()

	if err := write(f); err != nil {
		return fmt.Errorf("write mmdb: %w", err)
	}
	if err := f.Sync(); err != nil {
		return fmt.Errorf("sync mmdb: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close mmdb: %w", err)
	}
	db, err := maxminddb.Open(f.Name())
	if err != nil {
		return fmt.Errorf("open generated mmdb: %w", err)
	}
	verifyErr := db.Verify()
	closeErr := db.Close()
	if verifyErr != nil {
		return fmt.Errorf("verify generated mmdb: %w", verifyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close generated mmdb: %w", closeErr)
	}
	// Keep new files private, and restore an existing output's permissions only
	// after the temporary database is complete and validated.
	if previous != nil {
		if err := os.Chmod(f.Name(), previous.Mode().Perm()); err != nil {
			return fmt.Errorf("preserve output permissions: %w", err)
		}
	}
	if err := os.Rename(f.Name(), outPath); err != nil {
		return fmt.Errorf("replace output: %w", err)
	}
	return nil
}
