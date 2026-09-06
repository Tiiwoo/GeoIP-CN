package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigRejectsInvalid(t *testing.T) {
	for name, data := range map[string]string{
		"unknown field":  `{"sources":[{"type":"private"}],"output":{"file":"Country.mmdb","dir":"output","wantList":["private"]}}`,
		"no sources":     `{"output":{"file":"Country.mmdb","dir":"output","wantedList":["cn"]}}`,
		"no wanted list": `{"sources":[{"type":"private"}],"output":{"file":"Country.mmdb","dir":"output"}}`,
		"empty name":     `{"sources":[{"type":"private"}],"output":{"file":"Country.mmdb","dir":"output","wantedList":[" "]}}`,
		"no filename":    `{"sources":[{"type":"private"}],"output":{"dir":"output","wantedList":["private"]}}`,
		"no directory":   `{"sources":[{"type":"private"}],"output":{"file":"Country.mmdb","wantedList":["private"]}}`,
		"unknown type":   `{"sources":[{"type":"unknown"}],"output":{"file":"Country.mmdb","dir":"output","wantedList":["cn"]}}`,
		"no URL":         `{"sources":[{"type":"maxmind_mmdb"}],"output":{"file":"Country.mmdb","dir":"output","wantedList":["cn"]}}`,
		"no text name":   `{"sources":[{"type":"text","url":"ips.txt"}],"output":{"file":"Country.mmdb","dir":"output","wantedList":["cn"]}}`,
		"invalid family": `{"sources":[{"type":"text","name":"cn","url":"ips.txt","onlyIPType":"ip4"}],"output":{"file":"Country.mmdb","dir":"output","wantedList":["cn"]}}`,
		"null":           `null`,
		"trailing JSON":  `{"sources":[{"type":"private"}],"output":{"file":"Country.mmdb","dir":"output","wantedList":["private"]}} {}`,
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.json")
			if err := os.WriteFile(path, []byte(data), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := loadConfig(path); err == nil {
				t.Fatal("invalid config was accepted")
			}
		})
	}
}

func TestLoadDefaultConfig(t *testing.T) {
	if _, err := loadConfig("config.json"); err != nil {
		t.Fatal(err)
	}
}
