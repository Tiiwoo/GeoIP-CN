package main

import (
	"encoding/json"
	"os"
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
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
