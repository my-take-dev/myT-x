package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestAppTitleVersionMatchesWailsProductVersion(t *testing.T) {
	rawConfig, err := os.ReadFile("wails.json")
	if err != nil {
		t.Fatalf("ReadFile(wails.json) failed: %v", err)
	}

	var config struct {
		Info struct {
			ProductVersion string `json:"productVersion"`
		} `json:"info"`
	}
	if err := json.Unmarshal(rawConfig, &config); err != nil {
		t.Fatalf("Unmarshal(wails.json) failed: %v", err)
	}
	if config.Info.ProductVersion == "" {
		t.Fatal("wails.json productVersion must not be empty")
	}
	if !strings.Contains(appTitle, config.Info.ProductVersion) {
		t.Fatalf("appTitle %q must include wails.json productVersion %q", appTitle, config.Info.ProductVersion)
	}
}
