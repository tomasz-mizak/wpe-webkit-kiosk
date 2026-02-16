package tui

import (
	"os/exec"
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

// -- Messages --

type tickMsg time.Time
type refreshMsg struct {
	state    string
	url      string
	since    string
	cfgURL   string
	cfgInsp  string
	cfgHTTP  string
	vncState string
}
type actionDoneMsg struct{ text string }

// -- Model --

type mode int

const (
	modeNormal mode = iota
	modeInput
)

type model struct {
	state      string
	url        string
	since      string
	cfgURL     string
	cfgInsp    string
	cfgHTTP    string
	vncState   string
	message    string
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
		m.vncState = msg.vncState
		return m, nil

	case actionDoneMsg:
		m.message = msg.text
		return m, refreshCmd()

	case tea.KeyMsg:
		if m.mode == modeInput {
			return m.handleInput(msg)
		}
		return m.handleNormal(msg)
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
	}
	return m, nil
}

func (m model) handleInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		url := m.input
		m.mode = modeNormal
		m.input = ""
		if url == "" {
			m.message = ""
			return m, nil
		}
		m.message = "Opening " + url + "..."
		return m, openCmd(url)
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

	vncStr := inactiveStyle.Render("off")
	if m.vncState == "active" {
		vncStr = activeStyle.Render("on")
	}

	status := strings.Join([]string{
		labelStyle.Render("Service:") + "  " + stateStr,
		labelStyle.Render("Since:") + "  " + sinceStr,
		labelStyle.Render("URL:") + "  " + urlStr,
		labelStyle.Render("VNC:") + "  " + vncStr,
	}, "\n")

	cfg := strings.Join([]string{
		labelStyle.Render("URL:") + "  " + m.cfgURL,
		labelStyle.Render("Inspector:") + "  " + m.cfgInsp,
		labelStyle.Render("HTTP Insp.:") + "  " + m.cfgHTTP,
	}, "\n")

	statusPanel := panelStyle.Render(
		lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15")).Render("Status") + "\n\n" + status,
	)
	configPanel := panelStyle.Render(
		lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15")).Render("Config") + "\n\n" + cfg,
	)

	var msgLine string
	if m.mode == modeInput {
		msgLine = messageStyle.Render(m.message+" ") + m.input + "_"
	} else if m.message != "" {
		msgLine = messageStyle.Render(m.message)
	}

	help := helpStyle.Render("[o] open URL  [r] reload  [R] restart  [c] clear data  [v] toggle VNC  [q] quit")

	parts := []string{
		titleStyle.Render(" WPE WebKit Kiosk "),
		"",
		statusPanel,
		configPanel,
	}
	if msgLine != "" {
		parts = append(parts, "", msgLine)
	}
	parts = append(parts, "", help)

	return strings.Join(parts, "\n")
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
		}

		if out, err := exec.Command("systemctl", "show", vncServiceName,
			"--property=ActiveState", "--value").Output(); err == nil {
			msg.vncState = strings.TrimSpace(string(out))
		}

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

// Run starts the TUI dashboard.
func Run() error {
	p := tea.NewProgram(model{state: "loading..."}, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
