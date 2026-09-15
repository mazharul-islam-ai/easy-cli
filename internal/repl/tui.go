package repl

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"easy-cli/internal/app"
	"easy-cli/internal/config"
	"easy-cli/internal/ui"
)

type tuiModel struct {
	env        *IO
	input      textinput.Model
	spin       spinner.Model
	view       viewport.Model
	log        string
	status     string
	busy       bool
	width      int
	height     int
	quitting   bool
	cancel     context.CancelFunc
	confirmCh  chan bool
	confirming bool
}

type statusMsg string
type resultMsg string
type errMsg struct{ err error }
type confirmAskMsg struct {
	text string
	ch   chan bool
}

func newTUI(env *IO) tuiModel {
	in := textinput.New()
	in.Placeholder = "find invoices from last month"
	in.Prompt = ""
	in.CharLimit = 500
	in.Focus()
	in.Width = 60

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))

	vp := viewport.New(80, 12)
	intro := ui.HelpText() + "\n"
	vp.SetContent(intro)

	return tuiModel{
		env:    env,
		input:  in,
		spin:   sp,
		view:   vp,
		log:    intro,
		width:  80,
		height: 24,
	}
}

func (m tuiModel) Init() tea.Cmd {
	return tea.Batch(m.spin.Tick, textinput.Blink, tea.EnterAltScreen)
}

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.input.Width = max(20, msg.Width-8)
		m.layout()
		return m, nil
	case tea.KeyMsg:
		if m.confirming {
			return m.handleConfirmKey(msg)
		}
		if m.busy {
			if msg.Type == tea.KeyCtrlC {
				if m.cancel != nil {
					m.cancel()
				}
				m.status = "Cancelling…"
				return m, nil
			}
			if msg.Type == tea.KeyUp || msg.Type == tea.KeyDown || msg.Type == tea.KeyPgUp || msg.Type == tea.KeyPgDown {
				var cmd tea.Cmd
				m.view, cmd = m.view.Update(msg)
				return m, cmd
			}
			return m, nil
		}
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			m.quitting = true
			return m, tea.Quit
		case tea.KeyUp, tea.KeyDown, tea.KeyPgUp, tea.KeyPgDown:
			var cmd tea.Cmd
			m.view, cmd = m.view.Update(msg)
			return m, cmd
		case tea.KeyEnter:
			line := strings.TrimSpace(m.input.Value())
			m.input.SetValue("")
			if line == "" {
				return m, nil
			}
			return m.submit(line)
		}
	case spinner.TickMsg:
		if m.busy {
			var cmd tea.Cmd
			m.spin, cmd = m.spin.Update(msg)
			return m, cmd
		}
	case confirmAskMsg:
		m.confirming = true
		m.confirmCh = msg.ch
		m.status = "Send snippets to the cloud? y/N"
		m.appendLog(msg.text)
		m.layout()
		return m, nil
	case statusMsg:
		m.status = string(msg)
		return m, nil
	case resultMsg:
		m.finishWork()
		m.appendLog(string(msg))
		return m, nil
	case errMsg:
		m.finishWork()
		m.appendLog(ui.RenderError(msg.err))
		return m, nil
	}

	if !m.busy {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		cmds = append(cmds, cmd)
	}
	return m, tea.Batch(cmds...)
}

func (m *tuiModel) finishWork() {
	m.busy = false
	m.status = ""
	m.confirming = false
	m.confirmCh = nil
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
}

func (m tuiModel) handleConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC, tea.KeyEsc:
		m.replyConfirm(false)
		if m.cancel != nil {
			m.cancel()
		}
		m.status = "Cancelling…"
		return m, nil
	case tea.KeyEnter:
		m.replyConfirm(false)
		return m, nil
	}
	s := strings.ToLower(strings.TrimSpace(msg.String()))
	if s == "y" || s == "yes" {
		m.replyConfirm(true)
		m.status = "Sending snippets…"
		return m, m.spin.Tick
	}
	if s == "n" || s == "no" {
		m.replyConfirm(false)
		return m, nil
	}
	return m, nil
}

func (m *tuiModel) replyConfirm(ok bool) {
	if m.confirmCh != nil {
		select {
		case m.confirmCh <- ok:
		default:
		}
	}
	m.confirming = false
	m.confirmCh = nil
}

func (m *tuiModel) layout() {
	headerH := lipgloss.Height(m.headerView())
	footerH := lipgloss.Height(m.footerView())
	h := m.height - headerH - footerH
	if h < 3 {
		h = 3
	}
	m.view.Width = max(20, m.width-2)
	m.view.Height = h
	m.view.SetContent(m.log)
	m.view.GotoBottom()
}

func (m *tuiModel) appendLog(s string) {
	if s == "" {
		return
	}
	if m.log != "" && !strings.HasSuffix(m.log, "\n") {
		m.log += "\n"
	}
	m.log += s + "\n"
	m.view.SetContent(m.log)
	m.view.GotoBottom()
}

func (m tuiModel) submit(line string) (tea.Model, tea.Cmd) {
	shown := line
	cmd, rest, _ := strings.Cut(line, " ")
	cmd = strings.ToLower(strings.TrimSpace(cmd))
	rest = strings.TrimSpace(rest)
	switch cmd {
	case "exit", "quit":
		m.quitting = true
		return m, tea.Quit
	case "help", "?":
		m.appendLog(ui.RenderYou(shown))
		m.appendLog(ui.HelpText())
		return m, nil
	case "scope":
		m.appendLog(ui.RenderYou(shown))
		if rest == "" {
			m.appendLog(ui.RenderScope(m.env.Scopes))
			return m, nil
		}
		m.env.Scopes = strings.Fields(rest)
		m.appendLog(ui.RenderScope(m.env.Scopes))
		return m, nil
	case "provider":
		m.appendLog(ui.RenderYou(shown))
		name := "none"
		if m.env.App != nil && m.env.App.LLM != nil {
			name = m.env.App.LLM.Name()
		}
		m.appendLog("provider is set at startup (--provider / EASY_PROVIDER). current: " + name)
		return m, nil
	case "find", "search":
		line = rest
	}
	if line == "" {
		m.appendLog(ui.RenderYou(shown))
		m.appendLog(ui.RenderError(fmt.Errorf("empty query")))
		return m, nil
	}
	m.appendLog(ui.RenderYou(shown))
	m.busy = true
	m.status = "Working…"
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	m.cancel = cancel
	m.layout()
	return m, tea.Batch(m.spin.Tick, runWork(m.env, line, ctx))
}

func runWork(env *IO, line string, ctx context.Context) tea.Cmd {
	return func() tea.Msg {
		if env == nil || env.App == nil {
			return errMsg{fmt.Errorf("repl: nil app")}
		}
		if !env.App.AutoConfirm {
			env.App.Confirm = func(ctx context.Context, req app.ConfirmRequest) (bool, error) {
				ch := make(chan bool, 1)
				if tuiProg != nil {
					tuiProg.Send(confirmAskMsg{text: ui.RenderConfirm(req), ch: ch})
				} else {
					return false, app.ErrNeedYes
				}
				select {
				case ok := <-ch:
					if ok && req.FirstRun && env.ConfigHome != "" {
						_ = config.SavePrivacy(env.ConfigHome, config.Privacy{
							CloudSnippets: true,
							AcceptedAt:    time.Now().UTC().Format(time.RFC3339),
						})
					}
					return ok, nil
				case <-ctx.Done():
					return false, ctx.Err()
				}
			}
		}
		report := func(_ app.Stage, detail string) {
			if tuiProg != nil && detail != "" {
				tuiProg.Send(statusMsg(detail))
			}
		}
		if env.App.Last != nil && env.App.Last.CloudAllowed && isQuestion(line) {
			ans, err := env.App.AskWithProgress(ctx, line, report)
			if err != nil {
				return errMsg{err}
			}
			return resultMsg(ui.RenderAnswer(ans))
		}
		sess, err := env.App.SearchWithProgress(ctx, line, env.Scopes, report)
		if err != nil {
			return errMsg{err}
		}
		return resultMsg(ui.RenderSession(sess))
	}
}

var tuiProg *tea.Program

func (m tuiModel) headerView() string {
	scope := strings.Join(m.env.Scopes, " ")
	name := "none"
	if m.env != nil && m.env.App != nil && m.env.App.LLM != nil {
		name = m.env.App.LLM.Name()
	}
	return ui.Header(name, scope)
}

func (m tuiModel) footerView() string {
	var b strings.Builder
	if m.busy {
		b.WriteString(lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("214")).
			Padding(0, 1).
			Width(max(20, m.width-2)).
			Render(ui.StatusLine(m.spin.View(), m.status)))
		b.WriteString("\n")
	}
	focused := !m.busy
	if m.confirming {
		focused = false
	}
	b.WriteString(queryField(m.input.View(), m.width, focused))
	b.WriteString("\n")
	if m.confirming {
		b.WriteString(ui.HintBarConfirm())
	} else {
		b.WriteString(ui.HintBar())
	}
	return b.String()
}

func (m tuiModel) View() string {
	if m.quitting {
		return ""
	}
	header := m.headerView()
	footer := m.footerView()
	h := m.height - lipgloss.Height(header) - lipgloss.Height(footer)
	if h < 3 {
		h = 3
	}
	m.view.Width = max(20, m.width-2)
	m.view.Height = h
	return lipgloss.JoinVertical(lipgloss.Left, header, m.view.View(), footer)
}

func queryField(value string, width int, focused bool) string {
	if width < 40 {
		width = 40
	}
	st := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("51")).
		Padding(0, 1).
		Width(width - 2)
	if !focused {
		st = st.BorderForeground(lipgloss.Color("245"))
	}
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("51")).Render(" query ")
	return st.Render(title + "\n" + value)
}

func runTUI(env *IO) error {
	m := newTUI(env)
	p := tea.NewProgram(m, tea.WithInput(env.In), tea.WithOutput(env.Out))
	tuiProg = p
	defer func() { tuiProg = nil }()
	_, err := p.Run()
	return err
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
