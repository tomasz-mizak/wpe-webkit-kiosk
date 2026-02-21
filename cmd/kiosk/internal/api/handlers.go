package api

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/tomasz-mizak/wpe-webkit-kiosk/cmd/kiosk/internal/config"
	"github.com/tomasz-mizak/wpe-webkit-kiosk/cmd/kiosk/internal/dbus"
)

const kioskService = "wpe-webkit-kiosk"

// GET /status
func handleStatus(w http.ResponseWriter, r *http.Request) {
	state := systemctlProperty("ActiveState")

	var uptime *string
	if state == "active" {
		if v := systemctlProperty("ActiveEnterTimestamp"); v != "" {
			uptime = &v
		}
	}

	var url *string
	if client, err := dbus.NewClient(); err == nil {
		if u, err := client.GetUrl(); err == nil {
			url = &u
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"service": state,
		"uptime":  uptime,
		"url":     url,
	})
}

// POST /navigate
func handleNavigate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "Invalid JSON body")
		return
	}
	if body.URL == "" {
		writeError(w, http.StatusBadRequest, "invalid_body", "Field 'url' is required")
		return
	}

	client, err := dbus.NewClient()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "service_unavailable", "Kiosk service is not running")
		return
	}
	if err := client.Open(body.URL); err != nil {
		writeError(w, http.StatusInternalServerError, "dbus_error", err.Error())
		return
	}

	if cfg, err := config.Load(config.DefaultPath); err == nil {
		cfg.Set("URL", body.URL)
		cfg.Save()
	}

	writeJSON(w, http.StatusOK, map[string]string{"url": body.URL})
}

// POST /reload
func handleReload(w http.ResponseWriter, r *http.Request) {
	client, err := dbus.NewClient()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "service_unavailable", "Kiosk service is not running")
		return
	}
	if err := client.Reload(); err != nil {
		writeError(w, http.StatusInternalServerError, "dbus_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "reloaded"})
}

// GET /config
func handleConfigGet(w http.ResponseWriter, r *http.Request) {
	cfg, err := config.Load(config.DefaultPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "config_error", err.Error())
		return
	}

	result := make(map[string]string)
	for _, kv := range cfg.KeyValues() {
		if kv.Key == "API_TOKEN" {
			continue
		}
		result[kv.Key] = kv.Value
	}
	writeJSON(w, http.StatusOK, result)
}

// PUT /config
func handleConfigSet(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "Invalid JSON body")
		return
	}
	if body.Key == "" {
		writeError(w, http.StatusBadRequest, "invalid_body", "Field 'key' is required")
		return
	}
	if body.Key == "API_TOKEN" {
		writeError(w, http.StatusBadRequest, "forbidden_key", "API_TOKEN cannot be changed via this endpoint. Use kiosk api token regenerate")
		return
	}
	if !config.ValidKeys[body.Key] {
		writeError(w, http.StatusBadRequest, "unknown_key", fmt.Sprintf("Unknown config key: %s", body.Key))
		return
	}

	cfg, err := config.Load(config.DefaultPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "config_error", err.Error())
		return
	}

	cfg.Set(body.Key, body.Value)
	if err := cfg.Save(); err != nil {
		writeError(w, http.StatusInternalServerError, "config_error", err.Error())
		return
	}

	restartRequired := config.NeedsRestart(body.Key)

	if body.Key == "URL" {
		if client, err := dbus.NewClient(); err == nil {
			client.Open(body.Value)
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"key":              body.Key,
		"value":            body.Value,
		"restart_required": restartRequired,
	})
}

// POST /clear
func handleClear(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Scope string `json:"scope"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "Invalid JSON body")
		return
	}

	validScopes := map[string]bool{"cache": true, "cookies": true, "all": true}
	if !validScopes[body.Scope] {
		writeError(w, http.StatusBadRequest, "invalid_scope", "Valid scopes: cache, cookies, all")
		return
	}

	client, err := dbus.NewClient()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "service_unavailable", "Kiosk service is not running")
		return
	}
	if err := client.ClearData(body.Scope); err != nil {
		writeError(w, http.StatusInternalServerError, "dbus_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"cleared": body.Scope})
}

// GET /extensions
func handleExtensionsList(w http.ResponseWriter, r *http.Request) {
	exts, err := listExtensions()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "extensions_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, exts)
}

// POST /extensions/{name}/enable
func handleExtensionEnable(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "invalid_path", "Extension name is required")
		return
	}

	dir := getExtensionsDir()
	extDir := filepath.Join(dir, name)
	if _, err := os.Stat(filepath.Join(extDir, "manifest.json")); err != nil {
		writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("Extension %q not found", name))
		return
	}

	disabledPath := filepath.Join(extDir, ".disabled")
	os.Remove(disabledPath)
	exec.Command("sudo", "/usr/bin/rm", disabledPath).Run()

	writeJSON(w, http.StatusOK, map[string]string{"extension": name, "status": "enabled"})
}

// POST /extensions/{name}/disable
func handleExtensionDisable(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "invalid_path", "Extension name is required")
		return
	}

	dir := getExtensionsDir()
	extDir := filepath.Join(dir, name)
	if _, err := os.Stat(filepath.Join(extDir, "manifest.json")); err != nil {
		writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("Extension %q not found", name))
		return
	}

	disabledPath := filepath.Join(extDir, ".disabled")
	f, err := os.Create(disabledPath)
	if err != nil {
		exec.Command("sudo", "/usr/bin/touch", disabledPath).Run()
	} else {
		f.Close()
	}

	writeJSON(w, http.StatusOK, map[string]string{"extension": name, "status": "disabled"})
}

// POST /restart
func handleRestart(w http.ResponseWriter, r *http.Request) {
	if err := exec.Command("sudo", "systemctl", "restart", kioskService).Run(); err != nil {
		writeError(w, http.StatusInternalServerError, "restart_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "restarting"})
}

// GET /system
func handleSystem(w http.ResponseWriter, r *http.Request) {
	info := map[string]any{}

	if data, err := os.ReadFile("/proc/uptime"); err == nil {
		parts := strings.Fields(string(data))
		if len(parts) > 0 {
			if secs, err := strconv.ParseFloat(parts[0], 64); err == nil {
				info["uptime_seconds"] = secs
			}
		}
	}

	if data, err := os.ReadFile("/proc/loadavg"); err == nil {
		parts := strings.Fields(string(data))
		if len(parts) >= 3 {
			info["load_average"] = map[string]string{
				"1m": parts[0], "5m": parts[1], "15m": parts[2],
			}
		}
	}

	if data, err := os.ReadFile("/proc/meminfo"); err == nil {
		mem := parseMemInfo(string(data))
		info["memory"] = mem
	}

	if data, err := os.ReadFile("/proc/stat"); err == nil {
		cpu := parseCPUStat(string(data))
		info["cpu"] = cpu
	}

	info["disk"] = parseDiskUsage()
	info["network"] = parseNetworkInterfaces()
	info["temperature"] = parseTemperature()

	writeJSON(w, http.StatusOK, info)
}

// -- Helpers --

func systemctlProperty(prop string) string {
	out, err := exec.Command("systemctl", "show", kioskService,
		"--property="+prop, "--value").Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}

type extensionEntry struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Enabled bool   `json:"enabled"`
	DirName string `json:"dir_name"`
}

func getExtensionsDir() string {
	cfg, err := config.Load(config.DefaultPath)
	if err == nil {
		if dir := cfg.Get("EXTENSIONS_DIR"); dir != "" {
			return dir
		}
	}
	return config.DefaultExtensionsDir
}

func listExtensions() ([]extensionEntry, error) {
	dir := getExtensionsDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []extensionEntry{}, nil
		}
		return nil, err
	}

	var exts []extensionEntry
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name(), "manifest.json"))
		if err != nil {
			continue
		}
		var m struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		}
		if json.Unmarshal(data, &m) != nil || m.Name == "" {
			continue
		}
		_, disErr := os.Stat(filepath.Join(dir, entry.Name(), ".disabled"))
		exts = append(exts, extensionEntry{
			Name:    m.Name,
			Version: m.Version,
			Enabled: os.IsNotExist(disErr),
			DirName: entry.Name(),
		})
	}

	if exts == nil {
		exts = []extensionEntry{}
	}
	return exts, nil
}

func parseMemInfo(data string) map[string]int64 {
	result := map[string]int64{}
	scanner := bufio.NewScanner(strings.NewReader(data))
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		key := strings.TrimSuffix(parts[0], ":")
		switch key {
		case "MemTotal", "MemFree", "MemAvailable", "SwapTotal", "SwapFree":
			if val, err := strconv.ParseInt(parts[1], 10, 64); err == nil {
				result[key+"_kB"] = val
			}
		}
	}
	return result
}

func parseCPUStat(data string) map[string]string {
	scanner := bufio.NewScanner(strings.NewReader(data))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "cpu ") {
			fields := strings.Fields(line)
			if len(fields) >= 5 {
				return map[string]string{
					"user":   fields[1],
					"nice":   fields[2],
					"system": fields[3],
					"idle":   fields[4],
				}
			}
		}
	}
	return nil
}

func parseDiskUsage() []map[string]string {
	out, err := exec.Command("df", "-h", "--output=target,size,used,avail,pcent", "/").Output()
	if err != nil {
		return nil
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) < 2 {
		return nil
	}
	var disks []map[string]string
	for _, line := range lines[1:] {
		fields := strings.Fields(line)
		if len(fields) >= 5 {
			disks = append(disks, map[string]string{
				"mount": fields[0],
				"size":  fields[1],
				"used":  fields[2],
				"avail": fields[3],
				"use%":  fields[4],
			})
		}
	}
	return disks
}

func parseNetworkInterfaces() []map[string]string {
	data, err := os.ReadFile("/proc/net/dev")
	if err != nil {
		return nil
	}
	lines := strings.Split(string(data), "\n")
	var ifaces []map[string]string
	for _, line := range lines[2:] {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		name := strings.TrimSpace(parts[0])
		if name == "lo" {
			continue
		}
		fields := strings.Fields(parts[1])
		if len(fields) >= 9 {
			ifaces = append(ifaces, map[string]string{
				"name":     name,
				"rx_bytes": fields[0],
				"tx_bytes": fields[8],
			})
		}
	}
	return ifaces
}

func parseTemperature() []map[string]string {
	entries, err := os.ReadDir("/sys/class/thermal")
	if err != nil {
		return nil
	}
	var temps []map[string]string
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), "thermal_zone") {
			continue
		}
		base := filepath.Join("/sys/class/thermal", entry.Name())
		typeData, err := os.ReadFile(filepath.Join(base, "type"))
		if err != nil {
			continue
		}
		tempData, err := os.ReadFile(filepath.Join(base, "temp"))
		if err != nil {
			continue
		}
		millideg, err := strconv.ParseInt(strings.TrimSpace(string(tempData)), 10, 64)
		if err != nil {
			continue
		}
		temps = append(temps, map[string]string{
			"zone": strings.TrimSpace(string(typeData)),
			"temp": fmt.Sprintf("%.1f", float64(millideg)/1000.0),
		})
	}
	return temps
}
