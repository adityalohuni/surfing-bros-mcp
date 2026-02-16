package main

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/NimbleMarkets/ntcharts/canvas/runes"
	"github.com/NimbleMarkets/ntcharts/linechart/streamlinechart"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/harmonica"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"

	"github.com/adityalohuni/mcp-server/internal/admin"
	"github.com/adityalohuni/mcp-server/internal/adminclient"
	"github.com/adityalohuni/mcp-server/internal/config"
	"github.com/adityalohuni/mcp-server/internal/session"
)

type panel int
type uiMode int

const (
	clientsPanel panel = iota
	browsersPanel
)

const (
	dashboardMode uiMode = iota
	settingsMode
)

type loadResultMsg struct {
	clients []session.ClientInfo
	browser []admin.BrowserSession
	err     error
	at      time.Time
}

type disconnectResultMsg struct {
	target string
	id     string
	err    error
}

type serviceActionMsg struct {
	service string
	action  string
	cmd     *exec.Cmd
	err     error
}

type configSavedMsg struct {
	settings config.Settings
	err      error
}

type configReloadedMsg struct {
	settings config.Settings
	err      error
}

type tickMsg time.Time

type settingsForm struct {
	DaemonAddr      string
	MCPToken        string
	AdminToken      string
	ClientMaxIdle   string
	AdminBaseURL    string
	RefreshInterval string
}

type model struct {
	adminClient *adminclient.Client
	refresh     time.Duration
	repoRoot    string

	settings config.Settings
	form     settingsForm

	clients  []session.ClientInfo
	browsers []admin.BrowserSession

	mode           uiMode
	focus          panel
	clientCursor   int
	browserCursor  int
	settingsCursor int
	editingSetting bool

	editor textinput.Model

	mcpdCmd *exec.Cmd
	mcpCmd  *exec.Cmd
	mcpdLog string
	mcpLog  string

	spin spinner.Model

	clientVP  viewport.Model
	browserVP viewport.Model

	chartClients  streamlinechart.Model
	chartBrowsers streamlinechart.Model

	spring harmonica.Spring
	animC  float64
	animB  float64
	velC   float64
	velB   float64

	status      string
	lastUpdated time.Time
	width       int
	height      int
}

func newModel(client *adminclient.Client, refresh time.Duration, repoRoot string, cfg config.Settings) model {
	ed := textinput.New()
	ed.Prompt = "value> "
	ed.CharLimit = 512
	ed.Width = 64

	sp := spinner.New()
	sp.Spinner = spinner.MiniDot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))

	cChart := streamlinechart.New(
		34,
		8,
		streamlinechart.WithYRange(0, 32),
		streamlinechart.WithStyles(runes.ArcLineStyle, lipgloss.NewStyle().Foreground(lipgloss.Color("10"))),
	)
	bChart := streamlinechart.New(
		34,
		8,
		streamlinechart.WithYRange(0, 32),
		streamlinechart.WithStyles(runes.ArcLineStyle, lipgloss.NewStyle().Foreground(lipgloss.Color("14"))),
	)

	return model{
		adminClient:   client,
		refresh:       refresh,
		repoRoot:      repoRoot,
		settings:      cfg,
		form:          formFromSettings(cfg),
		mode:          dashboardMode,
		focus:         clientsPanel,
		status:        "loading...",
		mcpdLog:       filepath.Join(os.TempDir(), "surfingbro-mcpd.log"),
		mcpLog:        filepath.Join(os.TempDir(), "surfingbro-mcp.log"),
		spin:          sp,
		editor:        ed,
		clientVP:      viewport.New(40, 20),
		browserVP:     viewport.New(40, 20),
		chartClients:  cChart,
		chartBrowsers: bChart,
		spring:        harmonica.NewSpring(harmonica.FPS(60), 12.0, 1.0),
		animC:         0,
		animB:         0,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(fetchCmd(m.adminClient), tickCmd(m.refresh), m.spin.Tick)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.syncLayout()
		m.syncViewportContent()
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		return m, cmd

	case loadResultMsg:
		if msg.err != nil {
			m.status = "refresh failed: " + msg.err.Error()
			return m, nil
		}
		m.clients = msg.clients
		m.browsers = msg.browser
		sort.Slice(m.clients, func(i, j int) bool { return m.clients[i].ConnectedAt.Before(m.clients[j].ConnectedAt) })
		sort.Slice(m.browsers, func(i, j int) bool { return m.browsers[i].ConnectedAt.Before(m.browsers[j].ConnectedAt) })
		if m.clientCursor >= len(m.clients) {
			m.clientCursor = max(0, len(m.clients)-1)
		}
		if m.browserCursor >= len(m.browsers) {
			m.browserCursor = max(0, len(m.browsers)-1)
		}
		m.lastUpdated = msg.at
		m.chartClients.Push(float64(len(m.clients)))
		m.chartBrowsers.Push(float64(len(m.browsers)))
		m.chartClients.Draw()
		m.chartBrowsers.Draw()
		m.syncViewportContent()
		m.status = fmt.Sprintf("clients=%d browser_sessions=%d", len(m.clients), len(m.browsers))
		return m, nil

	case disconnectResultMsg:
		if msg.err != nil {
			m.status = fmt.Sprintf("disconnect %s %s failed: %v", msg.target, shortID(msg.id), msg.err)
			return m, nil
		}
		m.status = fmt.Sprintf("disconnected %s %s", msg.target, shortID(msg.id))
		return m, fetchCmd(m.adminClient)

	case serviceActionMsg:
		if msg.err != nil {
			m.status = fmt.Sprintf("%s %s failed: %v", msg.action, msg.service, msg.err)
			return m, nil
		}
		switch msg.service {
		case "mcpd":
			if msg.action == "start" {
				m.mcpdCmd = msg.cmd
			}
			if msg.action == "stop" {
				m.mcpdCmd = nil
			}
		case "mcp":
			if msg.action == "start" {
				m.mcpCmd = msg.cmd
			}
			if msg.action == "stop" {
				m.mcpCmd = nil
			}
		}
		m.status = fmt.Sprintf("%s %s ok", msg.action, msg.service)
		return m, fetchCmd(m.adminClient)

	case configReloadedMsg:
		if msg.err != nil {
			m.status = "config reload failed: " + msg.err.Error()
			return m, nil
		}
		m.settings = msg.settings
		m.form = formFromSettings(msg.settings)
		m.refresh = msg.settings.TUIRefreshInterval
		m.adminClient = adminclient.New(msg.settings.AdminBaseURL, msg.settings.AdminToken, &http.Client{Timeout: 4 * time.Second})
		m.status = "settings reloaded"
		return m, fetchCmd(m.adminClient)

	case configSavedMsg:
		if msg.err != nil {
			m.status = "save failed: " + msg.err.Error()
			return m, nil
		}
		m.settings = msg.settings
		m.form = formFromSettings(msg.settings)
		m.refresh = msg.settings.TUIRefreshInterval
		m.adminClient = adminclient.New(msg.settings.AdminBaseURL, msg.settings.AdminToken, &http.Client{Timeout: 4 * time.Second})
		m.status = "settings saved"
		return m, fetchCmd(m.adminClient)

	case tickMsg:
		if !procAlive(m.mcpdCmd) {
			m.mcpdCmd = nil
		}
		if !procAlive(m.mcpCmd) {
			m.mcpCmd = nil
		}
		m.animC, m.velC = m.spring.Update(m.animC, m.velC, float64(len(m.clients)))
		m.animB, m.velB = m.spring.Update(m.animB, m.velB, float64(len(m.browsers)))
		return m, tea.Batch(fetchCmd(m.adminClient), tickCmd(m.refresh))

	case tea.MouseMsg:
		if m.mode == dashboardMode && msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
			for i, c := range m.clients {
				if z := zone.Get("client-" + c.ID); z != nil && z.InBounds(msg) {
					m.focus = clientsPanel
					m.clientCursor = i
					m.syncViewportContent()
					return m, nil
				}
			}
			for i, b := range m.browsers {
				if z := zone.Get("browser-" + b.ID); z != nil && z.InBounds(msg) {
					m.focus = browsersPanel
					m.browserCursor = i
					m.syncViewportContent()
					return m, nil
				}
			}
		}

	case tea.KeyMsg:
		if m.mode == settingsMode {
			return updateSettingsMode(m, msg)
		}

		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "c":
			m.mode = settingsMode
			m.editingSetting = false
			m.editor.Blur()
			m.status = "settings mode"
			return m, nil
		case "tab":
			if m.focus == clientsPanel {
				m.focus = browsersPanel
			} else {
				m.focus = clientsPanel
			}
			m.syncViewportContent()
			return m, nil
		case "r":
			return m, fetchCmd(m.adminClient)
		case "up", "k":
			if m.focus == clientsPanel && m.clientCursor > 0 {
				m.clientCursor--
			}
			if m.focus == browsersPanel && m.browserCursor > 0 {
				m.browserCursor--
			}
			m.syncViewportContent()
			return m, nil
		case "down", "j":
			if m.focus == clientsPanel && m.clientCursor < len(m.clients)-1 {
				m.clientCursor++
			}
			if m.focus == browsersPanel && m.browserCursor < len(m.browsers)-1 {
				m.browserCursor++
			}
			m.syncViewportContent()
			return m, nil
		case "pgup":
			if m.focus == clientsPanel {
				m.clientVP.HalfViewUp()
			} else {
				m.browserVP.HalfViewUp()
			}
			return m, nil
		case "pgdown":
			if m.focus == clientsPanel {
				m.clientVP.HalfViewDown()
			} else {
				m.browserVP.HalfViewDown()
			}
			return m, nil
		case "d":
			if m.focus == clientsPanel && len(m.clients) > 0 {
				id := m.clients[m.clientCursor].ID
				return m, disconnectClientCmd(m.adminClient, id)
			}
			if m.focus == browsersPanel && len(m.browsers) > 0 {
				id := m.browsers[m.browserCursor].ID
				return m, disconnectBrowserCmd(m.adminClient, id)
			}
			return m, nil
		case "s":
			if procAlive(m.mcpdCmd) {
				m.status = "mcpd is already running"
				return m, nil
			}
			return m, startServiceCmd(m.repoRoot, "mcpd", m.mcpdLog)
		case "x":
			return m, stopServiceCmd("mcpd", m.mcpdCmd)
		case "m":
			if procAlive(m.mcpCmd) {
				m.status = "mcp is already running"
				return m, nil
			}
			return m, startServiceCmd(m.repoRoot, "mcp", m.mcpLog)
		case "n":
			return m, stopServiceCmd("mcp", m.mcpCmd)
		}
	}

	return m, nil
}

func updateSettingsMode(m model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.editingSetting {
		if msg.String() == "enter" {
			m.setSelectedSettingValue(m.editor.Value())
			m.editingSetting = false
			m.editor.Blur()
			m.status = "value updated (press s to save config)"
			return m, nil
		}
		if msg.String() == "esc" {
			m.editingSetting = false
			m.editor.Blur()
			m.status = "edit canceled"
			return m, nil
		}
		var cmd tea.Cmd
		m.editor, cmd = m.editor.Update(msg)
		return m, cmd
	}

	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "esc", "c":
		m.mode = dashboardMode
		m.status = "dashboard mode"
		return m, nil
	case "up", "k":
		if m.settingsCursor > 0 {
			m.settingsCursor--
		}
		return m, nil
	case "down", "j":
		if m.settingsCursor < len(settingNames())-1 {
			m.settingsCursor++
		}
		return m, nil
	case "r":
		return m, reloadConfigCmd(m.settings.Path)
	case "s":
		return m, saveConfigCmd(m.settings, m.form)
	case "e", "enter":
		m.editingSetting = true
		m.editor.SetValue(m.selectedSettingValue())
		m.editor.CursorEnd()
		cmd := m.editor.Focus()
		m.status = "editing " + settingNames()[m.settingsCursor]
		return m, cmd
	}
	return m, nil
}

func (m *model) syncLayout() {
	paneH := max(10, m.height-20)
	paneW := max(40, m.width/2-2)
	m.clientVP.Width = paneW - 2
	m.clientVP.Height = paneH
	m.browserVP.Width = paneW - 2
	m.browserVP.Height = paneH
}

func (m *model) syncViewportContent() {
	m.clientVP.SetContent(m.renderClientsRows())
	m.browserVP.SetContent(m.renderBrowserRows())
	m.ensureCursorVisible()
}

func (m *model) ensureCursorVisible() {
	if m.focus == clientsPanel {
		m.clientVP.GotoTop()
		if m.clientCursor > 0 {
			for i := 0; i < m.clientCursor; i++ {
				m.clientVP.LineDown(2)
			}
		}
		return
	}
	m.browserVP.GotoTop()
	if m.browserCursor > 0 {
		for i := 0; i < m.browserCursor; i++ {
			m.browserVP.LineDown(2)
		}
	}
}

func (m model) renderClientsRows() string {
	cursorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	normalStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	lines := make([]string, 0, len(m.clients)*2+1)
	if len(m.clients) == 0 {
		return normalStyle.Render("(none)")
	}
	for i, c := range m.clients {
		pref := "  "
		if i == m.clientCursor {
			pref = "> "
		}
		row := fmt.Sprintf("%s%s  %s  %s", pref, shortID(c.ID), emptyDefault(c.Name, "unnamed"), c.Transport)
		if i == m.clientCursor {
			row = cursorStyle.Render(row)
		}
		row = zone.Mark("client-"+c.ID, row)
		lines = append(lines, row)
		lines = append(lines, fmt.Sprintf("    %s  seen %s", c.RemoteAddr, timeAgo(c.LastSeen)))
	}
	return strings.Join(lines, "\n")
}

func (m model) renderBrowserRows() string {
	cursorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	activeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("14")).Bold(true)
	warnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	normalStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	lines := make([]string, 0, len(m.browsers)*3+1)
	if len(m.browsers) == 0 {
		return normalStyle.Render("(none)")
	}
	for i, s := range m.browsers {
		pref := "  "
		if i == m.browserCursor {
			pref = "> "
		}
		act := ""
		if s.Active {
			act = " " + activeStyle.Render("ACTIVE")
		}
		row := fmt.Sprintf("%s%s tabs=%d%s", pref, shortID(s.ID), len(s.Tabs), act)
		if i == m.browserCursor {
			row = cursorStyle.Render(row)
		}
		row = zone.Mark("browser-"+s.ID, row)
		lines = append(lines, row)
		lines = append(lines, fmt.Sprintf("    %s  seen %s", s.RemoteAddr, timeAgo(s.LastSeen)))
		if s.TabsError != "" {
			lines = append(lines, "    "+warnStyle.Render("tabs error: "+s.TabsError))
			continue
		}
		for _, tab := range s.Tabs {
			title := strings.TrimSpace(tab.Title)
			if title == "" {
				title = tab.URL
			}
			lines = append(lines, fmt.Sprintf("    - [%d] %s", tab.ID, trimText(title, 70)))
		}
	}
	return strings.Join(lines, "\n")
}

func (m model) View() string {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	normalStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))

	if m.mode == settingsMode {
		return zone.Scan(m.settingsView(titleStyle, normalStyle))
	}

	focusStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("10"))

	leftTitle := normalStyle.Render("MCP Clients")
	rightTitle := normalStyle.Render("Browser Sessions")
	if m.focus == clientsPanel {
		leftTitle = focusStyle.Render("MCP Clients")
	}
	if m.focus == browsersPanel {
		rightTitle = focusStyle.Render("Browser Sessions")
	}

	leftPane := lipgloss.NewStyle().Width(max(40, m.width/2-2)).Border(lipgloss.RoundedBorder()).Padding(0, 1).Render(leftTitle + "\n" + m.clientVP.View())
	rightPane := lipgloss.NewStyle().Width(max(40, m.width/2-2)).Border(lipgloss.RoundedBorder()).Padding(0, 1).Render(rightTitle + "\n" + m.browserVP.View())

	mcpdState := "down"
	if procAlive(m.mcpdCmd) {
		mcpdState = fmt.Sprintf("up pid=%d", m.mcpdCmd.Process.Pid)
	}
	mcpState := "down"
	if procAlive(m.mcpCmd) {
		mcpState = fmt.Sprintf("up pid=%d", m.mcpCmd.Process.Pid)
	}

	statC := int(math.Round(m.animC))
	statB := int(math.Round(m.animB))
	cards := lipgloss.JoinHorizontal(
		lipgloss.Top,
		lipgloss.NewStyle().Padding(0, 1).Border(lipgloss.RoundedBorder()).Render(fmt.Sprintf("Clients\n%d", statC)),
		lipgloss.NewStyle().Padding(0, 1).Border(lipgloss.RoundedBorder()).Render(fmt.Sprintf("Browsers\n%d", statB)),
		lipgloss.NewStyle().Padding(0, 1).Border(lipgloss.RoundedBorder()).Render(fmt.Sprintf("Updated\n%s", lastUpdatedText(m.lastUpdated))),
	)
	chartPanel := lipgloss.JoinHorizontal(lipgloss.Top,
		lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1).Render("Clients Trend\n"+m.chartClients.View()),
		lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1).Render("Browsers Trend\n"+m.chartBrowsers.View()),
	)

	help := normalStyle.Render("mouse: click row | tab panel | j/k move | pgup/pgdown scroll | d disconnect | r refresh | s/x mcpd | m/n mcp | c settings | q quit")
	proc := normalStyle.Render(fmt.Sprintf("mcpd[%s] %s | mcp[%s] %s | %s refreshing", mcpdState, m.mcpdLog, mcpState, m.mcpLog, m.spin.View()))
	status := titleStyle.Render("status: ") + m.status
	row := lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane)

	return zone.Scan(strings.Join([]string{
		titleStyle.Render("SurfingBro mpcd control"),
		cards,
		chartPanel,
		row,
		proc,
		status,
		help,
	}, "\n"))
}

func (m model) settingsView(titleStyle, normalStyle lipgloss.Style) string {
	cursorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)

	lines := []string{titleStyle.Render("Settings")}
	for i, name := range settingNames() {
		prefix := "  "
		if i == m.settingsCursor {
			prefix = cursorStyle.Render("> ")
		}
		lines = append(lines, fmt.Sprintf("%s%s = %s", prefix, name, m.settingValueByIndex(i)))
	}

	editLine := normalStyle.Render("select a field, press e or enter to edit")
	if m.editingSetting {
		editLine = keyStyle.Render("editing") + " " + settingNames()[m.settingsCursor] + "\n" + m.editor.View()
	}

	help := normalStyle.Render("j/k move | e/enter edit+apply | s save | r reload | c/esc back")
	status := titleStyle.Render("status: ") + m.status
	box := lipgloss.NewStyle().Width(max(80, m.width-2)).Border(lipgloss.RoundedBorder()).Padding(0, 1).Render(strings.Join(lines, "\n"))
	return strings.Join([]string{box, editLine, status, help}, "\n")
}

func fetchCmd(client *adminclient.Client) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		clients, err := client.ListClients(ctx)
		if err != nil {
			return loadResultMsg{err: err}
		}
		browsers, err := client.ListBrowsers(ctx)
		if err != nil {
			return loadResultMsg{err: err}
		}
		return loadResultMsg{clients: clients, browser: browsers, at: time.Now()}
	}
}

func disconnectClientCmd(client *adminclient.Client, id string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		err := client.DisconnectClient(ctx, id)
		return disconnectResultMsg{target: "client", id: id, err: err}
	}
}

func disconnectBrowserCmd(client *adminclient.Client, id string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		err := client.DisconnectBrowser(ctx, id)
		return disconnectResultMsg{target: "browser", id: id, err: err}
	}
}

func saveConfigCmd(current config.Settings, form settingsForm) tea.Cmd {
	return func() tea.Msg {
		next, err := formToSettings(current, form)
		if err != nil {
			return configSavedMsg{err: err}
		}
		saved, err := config.Save(next)
		if err != nil {
			return configSavedMsg{err: err}
		}
		return configSavedMsg{settings: saved}
	}
}

func reloadConfigCmd(path string) tea.Cmd {
	return func() tea.Msg {
		cfg, err := config.LoadOrCreate(path)
		if err != nil {
			return configReloadedMsg{err: err}
		}
		return configReloadedMsg{settings: cfg}
	}
}

func startServiceCmd(repoRoot, service, logPath string) tea.Cmd {
	return func() tea.Msg {
		logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
		if err != nil {
			return serviceActionMsg{service: service, action: "start", err: err}
		}
		cmd := exec.Command("go", "run", "./cmd/"+service)
		cmd.Dir = repoRoot
		cmd.Stdout = logFile
		cmd.Stderr = logFile
		if err := cmd.Start(); err != nil {
			_ = logFile.Close()
			return serviceActionMsg{service: service, action: "start", err: err}
		}
		go func() {
			_ = cmd.Wait()
			_ = logFile.Close()
		}()
		return serviceActionMsg{service: service, action: "start", cmd: cmd}
	}
}

func stopServiceCmd(service string, cmd *exec.Cmd) tea.Cmd {
	return func() tea.Msg {
		if !procAlive(cmd) {
			return serviceActionMsg{service: service, action: "stop", err: errors.New("not running")}
		}
		if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
			return serviceActionMsg{service: service, action: "stop", err: err}
		}
		return serviceActionMsg{service: service, action: "stop"}
	}
}

func tickCmd(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func procAlive(cmd *exec.Cmd) bool {
	if cmd == nil || cmd.Process == nil {
		return false
	}
	return cmd.Process.Signal(syscall.Signal(0)) == nil
}

func settingNames() []string {
	return []string{"daemon.addr", "auth.mcp_token", "auth.admin_token", "daemon.client_max_idle", "tui.admin_base_url", "tui.refresh_interval"}
}

func formFromSettings(s config.Settings) settingsForm {
	return settingsForm{
		DaemonAddr:      s.DaemonAddr,
		MCPToken:        s.MCPToken,
		AdminToken:      s.AdminToken,
		ClientMaxIdle:   s.ClientMaxIdle.String(),
		AdminBaseURL:    s.AdminBaseURL,
		RefreshInterval: s.TUIRefreshInterval.String(),
	}
}

func formToSettings(base config.Settings, form settingsForm) (config.Settings, error) {
	next := base
	next.DaemonAddr = strings.TrimSpace(form.DaemonAddr)
	next.MCPToken = strings.TrimSpace(form.MCPToken)
	next.AdminToken = strings.TrimSpace(form.AdminToken)
	next.AdminBaseURL = strings.TrimSpace(form.AdminBaseURL)
	if strings.TrimSpace(form.ClientMaxIdle) == "" {
		return config.Settings{}, errors.New("daemon.client_max_idle cannot be empty")
	}
	maxIdle, err := time.ParseDuration(strings.TrimSpace(form.ClientMaxIdle))
	if err != nil {
		return config.Settings{}, fmt.Errorf("invalid daemon.client_max_idle: %w", err)
	}
	if strings.TrimSpace(form.RefreshInterval) == "" {
		return config.Settings{}, errors.New("tui.refresh_interval cannot be empty")
	}
	refresh, err := time.ParseDuration(strings.TrimSpace(form.RefreshInterval))
	if err != nil {
		return config.Settings{}, fmt.Errorf("invalid tui.refresh_interval: %w", err)
	}
	next.ClientMaxIdle = maxIdle
	next.TUIRefreshInterval = refresh
	return next, nil
}

func (m model) selectedSettingValue() string { return m.settingValueByIndex(m.settingsCursor) }

func (m model) settingValueByIndex(i int) string {
	switch i {
	case 0:
		return m.form.DaemonAddr
	case 1:
		return m.form.MCPToken
	case 2:
		return m.form.AdminToken
	case 3:
		return m.form.ClientMaxIdle
	case 4:
		return m.form.AdminBaseURL
	case 5:
		return m.form.RefreshInterval
	default:
		return ""
	}
}

func (m *model) setSelectedSettingValue(value string) {
	switch m.settingsCursor {
	case 0:
		m.form.DaemonAddr = value
	case 1:
		m.form.MCPToken = value
	case 2:
		m.form.AdminToken = value
	case 3:
		m.form.ClientMaxIdle = value
	case 4:
		m.form.AdminBaseURL = value
	case 5:
		m.form.RefreshInterval = value
	}
}

func shortID(s string) string {
	if len(s) <= 8 {
		return s
	}
	return s[:8]
}

func emptyDefault(s, d string) string {
	if strings.TrimSpace(s) == "" {
		return d
	}
	return s
}

func timeAgo(t time.Time) string {
	if t.IsZero() {
		return "unknown"
	}
	d := time.Since(t).Round(time.Second)
	if d < 0 {
		d = 0
	}
	return d.String() + " ago"
}

func trimText(s string, n int) string {
	if n < 4 || len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}

func lastUpdatedText(t time.Time) string {
	if t.IsZero() {
		return "never"
	}
	return t.Format(time.RFC3339)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func findRepoRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	cur := wd
	for {
		if _, err := os.Stat(filepath.Join(cur, "go.mod")); err == nil {
			return cur, nil
		}
		next := filepath.Dir(cur)
		if next == cur {
			return "", errors.New("go.mod not found from cwd")
		}
		cur = next
	}
}

func main() {
	zone.NewGlobal()
	settings, err := config.LoadOrCreate("")
	if err != nil {
		fmt.Printf("config error: %v\n", err)
		return
	}
	repoRoot, err := findRepoRoot()
	if err != nil {
		fmt.Printf("repo error: %v\n", err)
		return
	}
	client := adminclient.New(settings.AdminBaseURL, settings.AdminToken, &http.Client{Timeout: 4 * time.Second})
	m := newModel(client, settings.TUIRefreshInterval, repoRoot, settings)
	m.syncLayout()
	m.syncViewportContent()
	if _, err := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion()).Run(); err != nil {
		fmt.Printf("tui error: %v\n", err)
	}
}
