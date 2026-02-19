package tui

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/tomasz-mizak/wpe-webkit-kiosk/cmd/kiosk/internal/config"
	"github.com/tomasz-mizak/wpe-webkit-kiosk/cmd/kiosk/internal/dbus"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const serviceName = "wpe-webkit-kiosk"
const vncServiceName = "wpe-webkit-kiosk-vnc"

// -- Styles --

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("15")).
			Background(lipgloss.Color("62")).
			Padding(0, 1)

	labelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245")).
			Width(14)

	colNameStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245")).
			Width(12)

	colHeaderStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245")).
			Bold(true).
			Width(12)

	activeStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("42")).
			Bold(true)

	inactiveStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Bold(true)

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	messageStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("220"))

	panelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(1, 2)
)

// -- Extension info --

type extInfo struct {
	dirName string
	name    string
	version string
	enabled bool
}

func scanExtensions() []extInfo {
	dir := config.DefaultExtensionsDir
	if cfg, err := config.Load(config.DefaultPath); err == nil {
		if d := cfg.Get("EXTENSIONS_DIR"); d != "" {
			dir = d
		}
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var exts []extInfo
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
		exts = append(exts, extInfo{
			dirName: entry.Name(),
			name:    m.Name,
			version: m.Version,
			enabled: os.IsNotExist(disErr),
		})
	}
	return exts
}

// -- Messages --

type tickMsg time.Time
type refreshMsg struct {
	state     string
	url       string
	since     string
	cfgURL    string
	cfgInsp   string
	cfgHTTP   string
	cfgVNC    string
	cfgCursor string
	cfgTTY    string
	exts      []extInfo
}
type actionDoneMsg struct{ text string }

// -- Model --

type mode int

const (
	modeNormal mode = iota
	modeInput
	modeInputTTY
	modeExtensions
)

type model struct {
	state     string
	url       string
	since     string
	cfgURL    string
	cfgInsp   string
	cfgHTTP   string
	cfgVNC    string
	cfgCursor string
	cfgTTY    string
	exts      []extInfo
	extCursor int
	message   string
	mode       mode
	input      string
	quitting   bool
	width      int
	height     int
}

func (m model) Init() tea.Cmd {
	return tea.Batch(tickCmd(), refreshCmd())
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tickMsg:
		return m, tea.Batch(tickCmd(), refreshCmd())

	case refreshMsg:
		m.state = msg.state
		m.url = msg.url
		m.since = msg.since
		m.cfgURL = msg.cfgURL
		m.cfgInsp = msg.cfgInsp
		m.cfgHTTP = msg.cfgHTTP
		m.cfgVNC = msg.cfgVNC
		m.cfgCursor = msg.cfgCursor
		m.cfgTTY = msg.cfgTTY
		m.exts = msg.exts
		if m.extCursor >= len(m.exts) {
			m.extCursor = 0
		}
		return m, nil

	case actionDoneMsg:
		m.message = msg.text
		return m, refreshCmd()

	case tea.KeyMsg:
		switch m.mode {
		case modeInput, modeInputTTY:
			return m.handleInput(msg)
		case modeExtensions:
			return m.handleExtensions(msg)
		default:
			return m.handleNormal(msg)
		}
	}
	return m, nil
}

func (m model) handleNormal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	case "o":
		m.mode = modeInput
		m.input = ""
		m.message = "Enter URL:"
		return m, nil
	case "r":
		m.message = "Reloading..."
		return m, reloadCmd()
	case "R":
		m.message = "Restarting service..."
		return m, restartCmd()
	case "c":
		m.message = "Clearing all data..."
		return m, clearDataCmd()
	case "v":
		m.message = "Toggling VNC..."
		return m, toggleVNCCmd()
	case "m":
		m.message = "Toggling cursor..."
		return m, toggleCursorCmd()
	case "t":
		m.mode = modeInputTTY
		m.input = ""
		m.message = "Enter TTY number (1-12):"
		return m, nil
	case "e":
		if len(m.exts) == 0 {
			m.message = "No extensions found"
			return m, nil
		}
		m.mode = modeExtensions
		m.message = ""
		return m, nil
	}
	return m, nil
}

func (m model) handleExtensions(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "e", "q":
		m.mode = modeNormal
		m.message = ""
		return m, nil
	case "up", "k":
		if m.extCursor > 0 {
			m.extCursor--
		}
		return m, nil
	case "down", "j":
		if m.extCursor < len(m.exts)-1 {
			m.extCursor++
		}
		return m, nil
	case "enter", " ":
		if m.extCursor < len(m.exts) {
			ext := m.exts[m.extCursor]
			m.message = "Toggling " + ext.name + "..."
			return m, toggleExtensionCmd(ext)
		}
		return m, nil
	}
	return m, nil
}

func (m model) handleInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		inputMode := m.mode
		value := m.input
		m.mode = modeNormal
		m.input = ""
		if value == "" {
			m.message = ""
			return m, nil
		}
		if inputMode == modeInputTTY {
			m.message = "Setting TTY to " + value + "..."
			return m, setTTYCmd(value)
		}
		m.message = "Opening " + value + "..."
		return m, openCmd(value)
	case "esc":
		m.mode = modeNormal
		m.input = ""
		m.message = ""
		return m, nil
	case "backspace":
		if len(m.input) > 0 {
			m.input = m.input[:len(m.input)-1]
		}
		return m, nil
	default:
		if len(msg.String()) == 1 || msg.String() == " " {
			m.input += msg.String()
		}
		return m, nil
	}
}

func (m model) View() string {
	if m.quitting {
		return ""
	}

	stateStr := inactiveStyle.Render(m.state)
	if m.state == "active" {
		stateStr = activeStyle.Render(m.state)
	}

	urlStr := m.url
	if urlStr == "" {
		urlStr = "-"
	}

	sinceStr := m.since
	if sinceStr == "" {
		sinceStr = "-"
	}

	status := strings.Join([]string{
		labelStyle.Render("Service:") + "  " + stateStr,
		labelStyle.Render("Since:") + "  " + sinceStr,
		labelStyle.Render("URL:") + "  " + urlStr,
	}, "\n")

	cfg := strings.Join([]string{
		labelStyle.Render("URL:") + "  " + m.cfgURL,
		labelStyle.Render("Inspector:") + "  " + m.cfgInsp,
		labelStyle.Render("HTTP Insp.:") + "  " + m.cfgHTTP,
	}, "\n")

	panelHeader := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15"))

	statusPanel := panelStyle.Render(
		panelHeader.Render("Status") + "\n\n" + status,
	)
	configPanel := panelStyle.Render(
		panelHeader.Render("Config") + "\n\n" + cfg,
	)

	featuresPanel := panelStyle.Render(
		panelHeader.Render("Features") + "\n\n" +
			colHeaderStyle.Render("Name") + colHeaderStyle.Render("Config") + "\n" +
			featureRow("VNC", m.cfgVNC, "true") + "\n" +
			featureRow("Cursor", m.cfgCursor, "true") + "\n" +
			colNameStyle.Render("TTY") + helpStyle.Render(m.cfgTTY),
	)

	var extRows []string
	if len(m.exts) == 0 {
		extRows = append(extRows, helpStyle.Render("  No extensions found"))
	} else {
		for i, ext := range m.exts {
			status := inactiveStyle.Render("disabled")
			if ext.enabled {
				status = activeStyle.Render("enabled")
			}
			prefix := "  "
			if m.mode == modeExtensions && i == m.extCursor {
				prefix = "> "
			}
			extRows = append(extRows,
				prefix+colNameStyle.Render(ext.dirName)+status+
					helpStyle.Render("  v"+ext.version))
		}
	}
	extTitle := panelHeader.Render("Extensions")
	if m.mode == modeExtensions {
		extTitle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("220")).Render("Extensions (select)")
	}
	extensionsPanel := panelStyle.Render(
		extTitle + "\n\n" + strings.Join(extRows, "\n"),
	)

	var msgLine string
	if m.mode == modeInput {
		msgLine = messageStyle.Render(m.message+" ") + m.input + "_"
	} else if m.message != "" {
		msgLine = messageStyle.Render(m.message)
	}

	var help string
	if m.mode == modeExtensions {
		help = helpStyle.Render("[↑/↓] select  [enter] toggle  [esc] back")
	} else {
		help = helpStyle.Render("[o] open URL  [r] reload  [R] restart  [c] clear data  [v] VNC  [m] cursor  [t] TTY  [e] extensions  [q] quit")
	}

	parts := []string{
		titleStyle.Render(" WPE WebKit Kiosk "),
		"",
		statusPanel,
		configPanel,
		featuresPanel,
		extensionsPanel,
	}
	if msgLine != "" {
		parts = append(parts, "", msgLine)
	}
	parts = append(parts, "", help)

	return strings.Join(parts, "\n")
}

func featureRow(name, cfgValue, enabledValue string) string {
	cfgStr := inactiveStyle.Render("disabled")
	if cfgValue == enabledValue {
		cfgStr = activeStyle.Render("enabled")
	}
	return colNameStyle.Render(name) + cfgStr
}

// -- Commands --

func tickCmd() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func refreshCmd() tea.Cmd {
	return func() tea.Msg {
		msg := refreshMsg{}

		if out, err := exec.Command("systemctl", "show", serviceName,
			"--property=ActiveState", "--value").Output(); err == nil {
			msg.state = strings.TrimSpace(string(out))
		} else {
			msg.state = "unknown"
		}

		if msg.state == "active" {
			if out, err := exec.Command("systemctl", "show", serviceName,
				"--property=ActiveEnterTimestamp", "--value").Output(); err == nil {
				msg.since = strings.TrimSpace(string(out))
			}
		}

		if client, err := dbus.NewClient(); err == nil {
			if url, err := client.GetUrl(); err == nil {
				msg.url = url
			}
		}

		if cfg, err := config.Load(config.DefaultPath); err == nil {
			msg.cfgURL = cfg.Get("URL")
			msg.cfgInsp = cfg.Get("INSPECTOR_PORT")
			msg.cfgHTTP = cfg.Get("INSPECTOR_HTTP_PORT")
			msg.cfgVNC = cfg.Get("VNC_ENABLED")
			msg.cfgCursor = cfg.Get("CURSOR_VISIBLE")
			msg.cfgTTY = cfg.Get("TTY")
		}

		msg.exts = scanExtensions()

		return msg
	}
}

func reloadCmd() tea.Cmd {
	return func() tea.Msg {
		client, err := dbus.NewClient()
		if err != nil {
			return actionDoneMsg{"Reload failed: " + err.Error()}
		}
		if err := client.Reload(); err != nil {
			return actionDoneMsg{"Reload failed: " + err.Error()}
		}
		return actionDoneMsg{"Page reloaded"}
	}
}

func restartCmd() tea.Cmd {
	return func() tea.Msg {
		if err := exec.Command("sudo", "systemctl", "restart", serviceName).Run(); err != nil {
			return actionDoneMsg{"Restart failed: " + err.Error()}
		}
		return actionDoneMsg{"Service restarted"}
	}
}

func openCmd(url string) tea.Cmd {
	return func() tea.Msg {
		client, err := dbus.NewClient()
		if err != nil {
			return actionDoneMsg{"Open failed: " + err.Error()}
		}
		if err := client.Open(url); err != nil {
			return actionDoneMsg{"Open failed: " + err.Error()}
		}

		if cfg, err := config.Load(config.DefaultPath); err == nil {
			cfg.Set("URL", url)
			cfg.Save()
		}

		return actionDoneMsg{"Navigated to " + url + " (saved)"}
	}
}

func clearDataCmd() tea.Cmd {
	return func() tea.Msg {
		client, err := dbus.NewClient()
		if err != nil {
			return actionDoneMsg{"Clear failed: " + err.Error()}
		}
		if err := client.ClearData("all"); err != nil {
			return actionDoneMsg{"Clear failed: " + err.Error()}
		}
		return actionDoneMsg{"All browsing data cleared"}
	}
}

func toggleVNCCmd() tea.Cmd {
	return func() tea.Msg {
		cfg, err := config.Load(config.DefaultPath)
		if err != nil {
			return actionDoneMsg{"VNC toggle failed: " + err.Error()}
		}

		current := cfg.Get("VNC_ENABLED")
		if current == "true" {
			cfg.Set("VNC_ENABLED", "false")
		} else {
			cfg.Set("VNC_ENABLED", "true")
		}

		if err := cfg.Save(); err != nil {
			return actionDoneMsg{"VNC toggle failed: " + err.Error()}
		}

		if err := exec.Command("sudo", "systemctl", "restart", vncServiceName).Run(); err != nil {
			return actionDoneMsg{"VNC config saved, but service restart failed: " + err.Error()}
		}

		if current == "true" {
			return actionDoneMsg{"VNC disabled"}
		}
		return actionDoneMsg{"VNC enabled"}
	}
}

func toggleCursorCmd() tea.Cmd {
	return func() tea.Msg {
		cfg, err := config.Load(config.DefaultPath)
		if err != nil {
			return actionDoneMsg{"Cursor toggle failed: " + err.Error()}
		}

		current := cfg.Get("CURSOR_VISIBLE")
		if current == "false" {
			cfg.Set("CURSOR_VISIBLE", "true")
		} else {
			cfg.Set("CURSOR_VISIBLE", "false")
		}

		if err := cfg.Save(); err != nil {
			return actionDoneMsg{"Cursor toggle failed: " + err.Error()}
		}

		if err := exec.Command("sudo", "systemctl", "restart", serviceName).Run(); err != nil {
			return actionDoneMsg{"Cursor config saved, but service restart failed: " + err.Error()}
		}

		if current == "false" {
			return actionDoneMsg{"Cursor enabled"}
		}
		return actionDoneMsg{"Cursor disabled"}
	}
}

func setTTYCmd(value string) tea.Cmd {
	return func() tea.Msg {
		n := 0
		for _, c := range value {
			if c < '0' || c > '9' {
				return actionDoneMsg{"Invalid TTY number: " + value}
			}
			n = n*10 + int(c-'0')
		}
		if n < 1 || n > 12 {
			return actionDoneMsg{"TTY must be between 1 and 12"}
		}

		cfg, err := config.Load(config.DefaultPath)
		if err != nil {
			return actionDoneMsg{"TTY set failed: " + err.Error()}
		}

		cfg.Set("TTY", value)
		if err := cfg.Save(); err != nil {
			return actionDoneMsg{"TTY set failed: " + err.Error()}
		}

		if err := exec.Command("sudo", "systemctl", "restart", serviceName).Run(); err != nil {
			return actionDoneMsg{"TTY set to " + value + ", but service restart failed: " + err.Error()}
		}
		return actionDoneMsg{"TTY set to " + value + " (service restarted)"}
	}
}

func toggleExtensionCmd(ext extInfo) tea.Cmd {
	return func() tea.Msg {
		dir := config.DefaultExtensionsDir
		if cfg, err := config.Load(config.DefaultPath); err == nil {
			if d := cfg.Get("EXTENSIONS_DIR"); d != "" {
				dir = d
			}
		}

		disabledPath := filepath.Join(dir, ext.dirName, ".disabled")

		var err error
		if ext.enabled {
			err = exec.Command("sudo", "/usr/bin/touch", disabledPath).Run()
		} else {
			err = exec.Command("sudo", "/usr/bin/rm", disabledPath).Run()
		}
		if err != nil {
			action := "enable"
			if ext.enabled {
				action = "disable"
			}
			return actionDoneMsg{"Failed to " + action + " " + ext.name + ": " + err.Error()}
		}

		action := "enabled"
		if ext.enabled {
			action = "disabled"
		}

		if err := exec.Command("sudo", "/usr/bin/systemctl", "restart", serviceName).Run(); err != nil {
			return actionDoneMsg{ext.name + " " + action + ", but restart failed: " + err.Error()}
		}
		return actionDoneMsg{ext.name + " " + action + " (service restarted)"}
	}
}

// Run starts the TUI dashboard.
func Run() error {
	p := tea.NewProgram(model{state: "loading..."}, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
