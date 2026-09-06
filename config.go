package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

type Config struct {
	Sources []Source `json:"sources"`
	Output  Output   `json:"output"`
}

type Source struct {
	Type       string   `json:"type"`
	Name       string   `json:"name,omitempty"`
	URL        string   `json:"url,omitempty"`
	WantedList []string `json:"wantedList,omitempty"`
	OnlyIPType string   `json:"onlyIPType,omitempty"`
}

type Output struct {
	File       string   `json:"file"`
	Dir        string   `json:"dir"`
	WantedList []string `json:"wantedList"`
}

func loadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil {
		return nil, err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, fmt.Errorf("config must contain exactly one JSON object")
	}
	if len(cfg.Sources) == 0 {
		return nil, fmt.Errorf("sources must not be empty")
	}
	for i, src := range cfg.Sources {
		if err := src.validate(); err != nil {
			return nil, fmt.Errorf("source %d: %w", i+1, err)
		}
	}
	if err := cfg.Output.validate(); err != nil {
		return nil, fmt.Errorf("output: %w", err)
	}
	return &cfg, nil
}

func (s Source) validate() error {
	switch s.Type {
	case "maxmind_mmdb", "ipinfo_mmdb", "text":
		if strings.TrimSpace(s.URL) == "" {
			return fmt.Errorf("url is required for %s", s.Type)
		}
	case "private":
	default:
		return fmt.Errorf("unknown source type %q", s.Type)
	}
	if s.Type == "text" && strings.TrimSpace(s.Name) == "" {
		return fmt.Errorf("name is required for text sources")
	}
	if s.OnlyIPType != "" {
		if s.Type != "text" || (s.OnlyIPType != "ipv4" && s.OnlyIPType != "ipv6") {
			return fmt.Errorf("onlyIPType must be ipv4 or ipv6 and is only supported for text sources")
		}
	}
	if len(s.WantedList) > 0 && s.Type != "maxmind_mmdb" && s.Type != "ipinfo_mmdb" {
		return fmt.Errorf("wantedList is only supported for MMDB sources")
	}
	return validateNames(s.WantedList)
}

func (o Output) validate() error {
	if strings.TrimSpace(o.File) == "" || strings.TrimSpace(o.Dir) == "" {
		return fmt.Errorf("file and dir are required")
	}
	if len(o.WantedList) == 0 {
		return fmt.Errorf("wantedList must not be empty")
	}
	return validateNames(o.WantedList)
}

func validateNames(names []string) error {
	seen := make(map[string]bool, len(names))
	for _, name := range names {
		name = strings.ToUpper(strings.TrimSpace(name))
		if name == "" {
			return fmt.Errorf("wantedList contains an empty name")
		}
		if seen[name] {
			return fmt.Errorf("wantedList contains duplicate name %q", name)
		}
		seen[name] = true
	}
	return nil
}
