package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testConfig = `# WPE Kiosk configuration
# After editing, restart the service: sudo systemctl restart wpe-webkit-kiosk

URL="https://wpewebkit.org"
INSPECTOR_PORT="8080"
INSPECTOR_HTTP_PORT="8090"
`

func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadParsesKeyValues(t *testing.T) {
	path := writeTempConfig(t, testConfig)
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	kvs := cfg.KeyValues()
	if len(kvs) != 3 {
		t.Fatalf("expected 3 key-value pairs, got %d", len(kvs))
	}
	if kvs[0].Key != "URL" || kvs[0].Value != "https://wpewebkit.org" {
		t.Errorf("unexpected first entry: %+v", kvs[0])
	}
	if kvs[1].Key != "INSPECTOR_PORT" || kvs[1].Value != "8080" {
		t.Errorf("unexpected second entry: %+v", kvs[1])
	}
	if kvs[2].Key != "INSPECTOR_HTTP_PORT" || kvs[2].Value != "8090" {
		t.Errorf("unexpected third entry: %+v", kvs[2])
	}
}

func TestGetReturnsValue(t *testing.T) {
	path := writeTempConfig(t, testConfig)
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	if got := cfg.Get("URL"); got != "https://wpewebkit.org" {
		t.Errorf("Get(URL) = %q, want %q", got, "https://wpewebkit.org")
	}
	if got := cfg.Get("MISSING"); got != "" {
		t.Errorf("Get(MISSING) = %q, want empty", got)
	}
}

func TestSetUpdatesExistingKey(t *testing.T) {
	path := writeTempConfig(t, testConfig)
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	cfg.Set("URL", "https://example.com")
	if got := cfg.Get("URL"); got != "https://example.com" {
		t.Errorf("after Set, Get(URL) = %q, want %q", got, "https://example.com")
	}
}

func TestSetAppendsNewKey(t *testing.T) {
	path := writeTempConfig(t, testConfig)
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	cfg.Set("NEW_KEY", "value")
	if got := cfg.Get("NEW_KEY"); got != "value" {
		t.Errorf("after Set, Get(NEW_KEY) = %q, want %q", got, "value")
	}
}

func TestSavePreservesComments(t *testing.T) {
	path := writeTempConfig(t, testConfig)
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	cfg.Set("URL", "https://changed.com")
	outPath := filepath.Join(t.TempDir(), "config-out")
	if err := cfg.SaveTo(outPath); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)

	if !strings.Contains(content, "# WPE Kiosk configuration") {
		t.Error("comment line was lost after save")
	}
	if !strings.Contains(content, `URL="https://changed.com"`) {
		t.Error("updated URL not found in saved file")
	}
	if !strings.Contains(content, `INSPECTOR_PORT="8080"`) {
		t.Error("INSPECTOR_PORT was lost after save")
	}
}

func TestNeedsRestart(t *testing.T) {
	if NeedsRestart("URL") {
		t.Error("URL should not need restart")
	}
	if !NeedsRestart("INSPECTOR_PORT") {
		t.Error("INSPECTOR_PORT should need restart")
	}
	if !NeedsRestart("INSPECTOR_HTTP_PORT") {
		t.Error("INSPECTOR_HTTP_PORT should need restart")
	}
}

func TestLoadEmptyFile(t *testing.T) {
	path := writeTempConfig(t, "")
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.KeyValues()) != 0 {
		t.Errorf("expected 0 key-value pairs, got %d", len(cfg.KeyValues()))
	}
}

func TestLoadCommentsOnly(t *testing.T) {
	path := writeTempConfig(t, "# just a comment\n# another\n")
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.KeyValues()) != 0 {
		t.Errorf("expected 0 key-value pairs from comments-only file, got %d", len(cfg.KeyValues()))
	}
}
