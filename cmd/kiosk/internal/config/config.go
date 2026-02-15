package config

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

const DefaultPath = "/etc/wpe-webkit-kiosk/config"

// LiveKeys can be applied at runtime without restarting the service.
var LiveKeys = map[string]bool{
	"URL": true,
}

// ValidKeys is the set of recognized configuration keys.
var ValidKeys = map[string]bool{
	"URL":                 true,
	"INSPECTOR_PORT":      true,
	"INSPECTOR_HTTP_PORT": true,
}

// Entry represents a single line in the config file.
type Entry struct {
	Raw   string // original line (for comments/blanks)
	Key   string // empty for non-KV lines
	Value string
}

// Config holds the parsed configuration file content.
type Config struct {
	Entries []Entry
	path    string
}

// Load parses the config file at the given path.
func Load(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("cannot open config: %w", err)
	}
	defer f.Close()

	var entries []Entry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		entry := Entry{Raw: line}

		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			if idx := strings.Index(trimmed, "="); idx > 0 {
				entry.Key = strings.TrimSpace(trimmed[:idx])
				val := strings.TrimSpace(trimmed[idx+1:])
				entry.Value = strings.Trim(val, "\"")
			}
		}
		entries = append(entries, entry)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading config: %w", err)
	}

	return &Config{Entries: entries, path: path}, nil
}

// Get returns the value for a key, or empty string if not found.
func (c *Config) Get(key string) string {
	for _, e := range c.Entries {
		if e.Key == key {
			return e.Value
		}
	}
	return ""
}

// Set updates or appends a key-value pair.
func (c *Config) Set(key, value string) {
	for i, e := range c.Entries {
		if e.Key == key {
			c.Entries[i].Value = value
			c.Entries[i].Raw = fmt.Sprintf("%s=\"%s\"", key, value)
			return
		}
	}
	c.Entries = append(c.Entries, Entry{
		Raw:   fmt.Sprintf("%s=\"%s\"", key, value),
		Key:   key,
		Value: value,
	})
}

// KeyValues returns all key-value pairs in order.
func (c *Config) KeyValues() []Entry {
	var kvs []Entry
	for _, e := range c.Entries {
		if e.Key != "" {
			kvs = append(kvs, e)
		}
	}
	return kvs
}

// Save writes the config back to disk, preserving comments and blank lines.
func (c *Config) Save() error {
	return c.SaveTo(c.path)
}

// SaveTo writes the config to the specified path.
func (c *Config) SaveTo(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("cannot write config: %w", err)
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	for _, e := range c.Entries {
		fmt.Fprintln(w, e.Raw)
	}
	return w.Flush()
}

// NeedsRestart returns true if changing the given key requires a service restart.
func NeedsRestart(key string) bool {
	return !LiveKeys[key]
}
