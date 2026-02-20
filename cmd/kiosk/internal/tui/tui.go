package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/tomasz-mizak/wpe-webkit-kiosk/cmd/kiosk/internal/audio"
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

	activeTabStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("15")).
			Background(lipgloss.Color("62")).
			Padding(0, 1)

	inactiveTabStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("245")).
				Padding(0, 1)

	labelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245")).
			Width(16)

	activeStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("42")).
			Bold(true)

	inactiveStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Bold(true)

	cursorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("220")).
			Bold(true)

	actionLabelStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("75"))

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	messageStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("220"))
)

// -- Tabs --

type tab int

const (
	tabStatus tab = iota
	tabConfig
	tabFeatures
	tabExtensions
	tabCount
)

var tabNames = [tabCount]string{"Status", "Config", "Features", "Extensions"}

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
	volume    int
	muted     bool
	audioErr  bool
	exts      []extInfo
}
type actionDoneMsg struct{ text string }

// -- Mode --

type mode int

const (
	modeNormal mode = iota
	modeEdit
	modeVolume
)

// -- Model --

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
	volume    int
	muted     bool
	audioErr  bool
	exts      []extInfo

	activeTab  tab
	tabCursors [tabCount]int

	mode      mode
	editField string
	input     string

	message  string
	quitting bool
	width    int
	height   int
}

func (m model) tabItemCount() int {
	switch m.activeTab {
	case tabStatus:
		return 6
	case tabConfig:
		return 3
	case tabFeatures:
		return 4
	case tabExtensions:
		return len(m.exts)
	}
	return 0
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
		m.volume = msg.volume
		m.muted = msg.muted
		m.audioErr = msg.audioErr
		m.exts = msg.exts
		if m.tabCursors[tabExtensions] >= len(m.exts) && len(m.exts) > 0 {
			m.tabCursors[tabExtensions] = len(m.exts) - 1
		}
		if len(m.exts) == 0 {
			m.tabCursors[tabExtensions] = 0
		}
		return m, nil

	case actionDoneMsg:
		m.message = msg.text
		return m, refreshCmd()

	case tea.KeyMsg:
		if m.mode == modeEdit {
			return m.handleEdit(msg)
		}
		if m.mode == modeVolume {
			return m.handleVolume(msg)
		}
		return m.handleNormal(msg)
	}
	return m, nil
}

// -- Key handlers --

func (m model) handleNormal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		m.quitting = true
		return m, tea.Quit

	case "left", "h":
		if m.activeTab == 0 {
			m.activeTab = tabCount - 1
		} else {
			m.activeTab--
		}
		return m, nil

	case "right", "l":
		m.activeTab++
		if m.activeTab >= tabCount {
			m.activeTab = 0
		}
		return m, nil

	case "up", "k":
		if m.tabCursors[m.activeTab] > 0 {
			m.tabCursors[m.activeTab]--
		}
		return m, nil

	case "down", "j":
		max := m.tabItemCount() - 1
		if max < 0 {
			max = 0
		}
		if m.tabCursors[m.activeTab] < max {
			m.tabCursors[m.activeTab]++
		}
		return m, nil

	case "enter":
		return m.handleActivate()
	}
	return m, nil
}

func (m model) handleActivate() (tea.Model, tea.Cmd) {
	cursor := m.tabCursors[m.activeTab]
	switch m.activeTab {
	case tabStatus:
		switch cursor {
		case 3:
			m.message = "Reloading..."
			return m, reloadCmd()
		case 4:
			m.message = "Restarting service..."
			return m, restartCmd()
		case 5:
			m.message = "Clearing all data..."
			return m, clearDataCmd()
		}
	case tabConfig:
		if cursor == 0 {
			m.mode = modeEdit
			m.editField = "url"
			m.input = m.cfgURL
			return m, nil
		}
	case tabFeatures:
		switch cursor {
		case 0:
			m.message = "Toggling VNC..."
			return m, toggleVNCCmd()
		case 1:
			m.message = "Toggling cursor..."
			return m, toggleCursorCmd()
		case 2:
			m.mode = modeEdit
			m.editField = "tty"
			m.input = m.cfgTTY
			return m, nil
		case 3:
			if m.audioErr {
				m.message = "No sound card detected"
				return m, nil
			}
			m.mode = modeVolume
			m.message = "Volume: [↑/↓] adjust  [m] mute  [esc] back"
			return m, nil
		}
	case tabExtensions:
		if cursor < len(m.exts) {
			ext := m.exts[cursor]
			m.message = "Toggling " + ext.name + "..."
			return m, toggleExtensionCmd(ext)
		}
	}
	return m, nil
}

func (m model) handleVolume(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.mode = modeNormal
		m.message = ""
		return m, nil
	case "up", "right", "+", "=":
		newLevel := m.volume + 5
		if newLevel > 100 {
			newLevel = 100
		}
		return m, volumeSetCmd(newLevel)
	case "down", "left", "-":
		newLevel := m.volume - 5
		if newLevel < 0 {
			newLevel = 0
		}
		return m, volumeSetCmd(newLevel)
	case "m":
		return m, volumeToggleMuteCmd()
	}
	return m, nil
}

func (m model) handleEdit(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		value := m.input
		field := m.editField
		m.mode = modeNormal
		m.input = ""
		m.editField = ""
		if value == "" {
			m.message = ""
			return m, nil
		}
		switch field {
		case "url":
			m.message = "Opening " + value + "..."
			return m, openCmd(value)
		case "tty":
			m.message = "Setting TTY to " + value + "..."
			return m, setTTYCmd(value)
		}
		return m, nil
	case "esc":
		m.mode = modeNormal
		m.input = ""
		m.editField = ""
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

// -- View --

func (m model) View() string {
	if m.quitting {
		return ""
	}

	var b strings.Builder

	// Header
	b.WriteString(titleStyle.Render(" wpe-kiosk "))
	b.WriteString("\n\n")

	// Tab bar
	for i := tab(0); i < tabCount; i++ {
		if i == m.activeTab {
			b.WriteString(activeTabStyle.Render(tabNames[i]))
		} else {
			b.WriteString(inactiveTabStyle.Render(tabNames[i]))
		}
		if i < tabCount-1 {
			b.WriteString("  ")
		}
	}
	b.WriteString("\n\n")

	// Tab content
	switch m.activeTab {
	case tabStatus:
		m.renderStatusTab(&b)
	case tabConfig:
		m.renderConfigTab(&b)
	case tabFeatures:
		m.renderFeaturesTab(&b)
	case tabExtensions:
		m.renderExtensionsTab(&b)
	}

	// Status bar
	b.WriteString("\n")
	stateIndicator := inactiveStyle.Render("● " + m.state)
	if m.state == "active" {
		stateIndicator = activeStyle.Render("● " + m.state)
	}
	statusLine := stateIndicator
	if m.message != "" {
		statusLine += "  " + messageStyle.Render(m.message)
	}
	b.WriteString(statusLine)
	b.WriteString("\n")

	// Help bar
	var help string
	switch m.mode {
	case modeEdit:
		help = "[enter] confirm  [esc] cancel"
	case modeVolume:
		help = "[↑/↓] adjust  [m] mute/unmute  [esc] back"
	default:
		help = "[←/→] tab  [↑/↓] select  [enter] activate  [q] quit"
	}
	b.WriteString(helpStyle.Render(help))

	return b.String()
}

func (m model) renderInfoRow(b *strings.Builder, index int, label, value string) {
	cursor := m.tabCursors[m.activeTab]
	if index == cursor {
		b.WriteString(cursorStyle.Render("> "))
	} else {
		b.WriteString("  ")
	}
	b.WriteString(labelStyle.Render(label))
	b.WriteString(value)
	b.WriteString("\n")
}

func (m model) renderActionRow(b *strings.Builder, index int, label string) {
	cursor := m.tabCursors[m.activeTab]
	if index == cursor {
		b.WriteString(cursorStyle.Render("> "))
	} else {
		b.WriteString("  ")
	}
	b.WriteString(actionLabelStyle.Render(label))
	b.WriteString("\n")
}

func (m model) renderStatusTab(b *strings.Builder) {
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

	m.renderInfoRow(b, 0, "Service", stateStr)
	m.renderInfoRow(b, 1, "Uptime", sinceStr)
	m.renderInfoRow(b, 2, "URL", urlStr)
	b.WriteString("\n")
	m.renderActionRow(b, 3, "Reload page")
	m.renderActionRow(b, 4, "Restart service")
	m.renderActionRow(b, 5, "Clear all data")
}

func (m model) renderConfigTab(b *strings.Builder) {
	urlValue := m.cfgURL
	if m.mode == modeEdit && m.editField == "url" {
		urlValue = m.input + "_"
	}

	m.renderInfoRow(b, 0, "URL", urlValue)
	m.renderInfoRow(b, 1, "Inspector", m.cfgInsp)
	m.renderInfoRow(b, 2, "HTTP Insp.", m.cfgHTTP)
}

func (m model) renderFeaturesTab(b *strings.Builder) {
	vncStr := inactiveStyle.Render("disabled")
	if m.cfgVNC == "true" {
		vncStr = activeStyle.Render("enabled")
	}
	cursorStr := inactiveStyle.Render("disabled")
	if m.cfgCursor == "true" {
		cursorStr = activeStyle.Render("enabled")
	}

	ttyStr := m.cfgTTY
	if m.mode == modeEdit && m.editField == "tty" {
		ttyStr = m.input + "_"
	}

	var volumeStr string
	if m.audioErr {
		volumeStr = helpStyle.Render("no sound card")
	} else if m.muted {
		volumeStr = inactiveStyle.Render(fmt.Sprintf("muted (%d%%)", m.volume))
	} else {
		filled := m.volume / 10
		empty := 10 - filled
		bar := strings.Repeat("█", filled) + strings.Repeat("░", empty)
		volumeStr = activeStyle.Render(fmt.Sprintf("%s %d%%", bar, m.volume))
	}

	m.renderInfoRow(b, 0, "VNC", vncStr)
	m.renderInfoRow(b, 1, "Cursor", cursorStr)
	m.renderInfoRow(b, 2, "TTY", ttyStr)
	m.renderInfoRow(b, 3, "Volume", volumeStr)
}

func (m model) renderExtensionsTab(b *strings.Builder) {
	if len(m.exts) == 0 {
		b.WriteString(helpStyle.Render("  No extensions found"))
		b.WriteString("\n")
		return
	}
	cursor := m.tabCursors[m.activeTab]
	for i, ext := range m.exts {
		if i == cursor {
			b.WriteString(cursorStyle.Render("> "))
		} else {
			b.WriteString("  ")
		}
		statusStr := inactiveStyle.Render("disabled")
		if ext.enabled {
			statusStr = activeStyle.Render("enabled")
		}
		b.WriteString(labelStyle.Render(ext.dirName))
		b.WriteString(statusStr)
		b.WriteString(helpStyle.Render("  v" + ext.version))
		b.WriteString("\n")
	}
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

		if level, muted, err := audio.GetVolume(); err == nil {
			msg.volume = level
			msg.muted = muted
		} else {
			msg.audioErr = true
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

func volumeSetCmd(level int) tea.Cmd {
	return func() tea.Msg {
		if err := audio.SetVolume(level); err != nil {
			return actionDoneMsg{fmt.Sprintf("Volume failed: %s", err)}
		}
		return actionDoneMsg{fmt.Sprintf("Volume: %d%%", level)}
	}
}

func volumeToggleMuteCmd() tea.Cmd {
	return func() tea.Msg {
		if err := audio.ToggleMute(); err != nil {
			return actionDoneMsg{fmt.Sprintf("Mute toggle failed: %s", err)}
		}
		return actionDoneMsg{"Mute toggled"}
	}
}

// Run starts the TUI dashboard.
func Run() error {
	p := tea.NewProgram(model{state: "loading..."}, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
