package main

import (
	"bytes"
	"errors"
	"io"
	"net/netip"
	"os"
	"path/filepath"
	"testing"

	"github.com/maxmind/mmdbwriter/mmdbtype"
	maxminddb "github.com/oschwald/maxminddb-golang/v2"
)

func lookupCountry(t *testing.T, db *maxminddb.Reader, ip string) (string, bool) {
	t.Helper()
	var record struct {
		Country struct {
			ISO string `maxminddb:"iso_code"`
		} `maxminddb:"country"`
	}
	result := db.Lookup(netip.MustParseAddr(ip))
	if err := result.Decode(&record); err != nil {
		t.Fatal(err)
	}
	return record.Country.ISO, result.Found()
}

func TestWriteMMDBRejectsMissingOrEmptyEntries(t *testing.T) {
	for _, tc := range []struct {
		name    string
		entries map[string]*Entry
		wanted  []string
	}{
		{"missing country", map[string]*Entry{}, []string{"cn"}},
		{"empty country", map[string]*Entry{"CN": NewEntry("cn")}, []string{"cn"}},
		{"missing wanted list", map[string]*Entry{}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "Country.mmdb")
			old := []byte("previous output")
			if err := os.WriteFile(path, old, 0644); err != nil {
				t.Fatal(err)
			}
			if err := writeMMDB(tc.entries, Output{File: "Country.mmdb", Dir: dir, WantedList: tc.wanted}); err == nil {
				t.Error("invalid output was accepted")
			}
			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, old) {
				t.Error("previous output was overwritten")
			}
		})
	}
}

func TestWriteMMDBRoundTrip(t *testing.T) {
	cn := NewEntry("cn")
	for _, p := range []string{"1.0.1.0/25", "1.0.1.128/25", "2400:3200::/32"} {
		if err := cn.AddPrefix(p); err != nil {
			t.Fatal(err)
		}
	}
	priv, err := privateEntry()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := writeMMDB(map[string]*Entry{"CN": cn, "PRIVATE": priv}, Output{File: "Country.mmdb", Dir: dir, WantedList: []string{"cn", "private"}}); err != nil {
		t.Fatal(err)
	}
	db, err := maxminddb.Open(filepath.Join(dir, "Country.mmdb"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, tc := range []struct{ ip, want string }{{"1.0.1.1", "CN"}, {"1.0.1.254", "CN"}, {"2400:3200::1", "CN"}, {"10.0.0.1", "PRIVATE"}, {"fc00::1", "PRIVATE"}, {"8.8.8.8", ""}} {
		got, found := lookupCountry(t, db, tc.ip)
		if got != tc.want || found != (tc.want != "") {
			t.Errorf("lookup %s = %q/%t, want %q", tc.ip, got, found, tc.want)
		}
	}
}

func TestWriteMMDBFilePreservesOldOutputOnFailure(t *testing.T) {
	for _, mode := range []string{"partial write", "invalid database"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "Country.mmdb")
			old := []byte("previous output")
			if err := os.WriteFile(path, old, 0644); err != nil {
				t.Fatal(err)
			}
			failure := errors.New("injected write failure")
			err := writeMMDBFile(path, func(w io.Writer) error {
				if _, err := w.Write([]byte("partial new database")); err != nil {
					return err
				}
				if mode == "partial write" {
					return failure
				}
				return nil
			})
			if err == nil {
				t.Fatal("write or validation failure was ignored")
			}
			if mode == "partial write" && !errors.Is(err, failure) {
				t.Errorf("error = %v, want write failure", err)
			}
			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, old) {
				t.Fatal("previous output was damaged")
			}
			files, err := os.ReadDir(dir)
			if err != nil {
				t.Fatal(err)
			}
			if len(files) != 1 {
				t.Fatalf("temporary file was not cleaned up: %v", files)
			}
		})
	}
}

func TestWriteMMDBFilePreservesPermissions(t *testing.T) {
	data := makeMMDB(t, map[string]mmdbtype.Map{"1.0.1.0/24": {"country": mmdbtype.Map{"iso_code": mmdbtype.String("CN")}}})
	for _, mode := range []os.FileMode{0, 0600, 0640} {
		t.Run(mode.String(), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "Country.mmdb")
			if mode != 0 {
				if err := os.WriteFile(path, []byte("old database"), mode); err != nil {
					t.Fatal(err)
				}
				if err := os.Chmod(path, mode); err != nil {
					t.Fatal(err)
				}
			}
			if err := writeMMDBFile(path, func(w io.Writer) error {
				info, err := w.(*os.File).Stat()
				if err != nil {
					return err
				}
				if info.Mode().Perm()&0077 != 0 {
					t.Error("temporary database is readable by other users")
				}
				_, err = w.Write(data)
				return err
			}); err != nil {
				t.Fatal(err)
			}
			info, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			want := mode
			if want == 0 {
				want = 0600
			}
			if got := info.Mode().Perm(); got != want {
				t.Errorf("output mode = %o, want %o", got, want)
			}
		})
	}
}
