package main

import (
	"fmt"
	"math"
	"math/rand"
	"strings"
	"time"
	"unicode"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type screenState int

const (
	stateReady screenState = iota
	stateRunning
	stateFinished
)

type tickMsg time.Time

type keyMap struct {
	quit    key.Binding
	restart key.Binding
}

type model struct {
	words     []string
	rng       *rand.Rand
	session   typingSession
	state     screenState
	startedAt time.Time
	remaining time.Duration
	width     int
	height    int
	help      help.Model
	keys      keyMap
}

func newModel(words []string, rng *rand.Rand) model {
	m := model{
		words:     append([]string(nil), words...),
		rng:       rng,
		state:     stateReady,
		remaining: testDuration,
		help:      help.New(),
		keys: keyMap{
			quit: key.NewBinding(
				key.WithKeys("q", "ctrl+c"),
				key.WithHelp("q", "quit"),
			),
			restart: key.NewBinding(
				key.WithKeys("r"),
				key.WithHelp("r", "restart"),
			),
		},
	}

	m.help.ShowAll = false
	m.resetSession()

	return m
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tickMsg:
		if m.state != stateRunning {
			return m, nil
		}

		elapsed := time.Time(msg).Sub(m.startedAt)
		if elapsed >= testDuration {
			m.remaining = 0
			m.state = stateFinished
			return m, nil
		}

		m.remaining = testDuration - elapsed
		return m, tickCmd()

	case tea.KeyMsg:
		if key.Matches(msg, m.keys.quit) {
			return m, tea.Quit
		}

		if m.state == stateFinished {
			if key.Matches(msg, m.keys.restart) {
				m.resetSession()
			}

			return m, nil
		}

		switch msg.Type {
		case tea.KeyBackspace, tea.KeyCtrlH:
			m.session.Backspace()
			return m, nil
		}

		if len(msg.Runes) != 1 || !unicode.IsPrint(msg.Runes[0]) {
			return m, nil
		}

		cmd := tea.Cmd(nil)
		if m.state == stateReady {
			m.state = stateRunning
			m.startedAt = time.Now()
			m.remaining = testDuration
			cmd = tickCmd()
		}

		m.session.TypeRune(msg.Runes[0])
		return m, cmd
	}

	return m, nil
}

func (m model) View() string {
	width := m.width
	if width == 0 {
		width = 100
	}

	contentWidth := maxInt(50, width-6)
	if width < 56 {
		contentWidth = maxInt(20, width-2)
	}

	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#F8FAFC")).
		Render("GoTap")

	subtitle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#94A3B8"))

	statusLine := subtitle.Render(m.statusCopy())
	prompt := m.renderPrompt(contentWidth - 4)
	promptBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#334155")).
		Padding(1, 2).
		Width(contentWidth).
		Render(prompt)

	statsRow := lipgloss.JoinHorizontal(
		lipgloss.Top,
		m.renderStat("Time", fmt.Sprintf("%.1fs", m.remainingSeconds())),
		m.renderStat("Chars", fmt.Sprintf("%d", m.session.charsTyped)),
		m.renderStat("Accuracy", fmt.Sprintf("%.1f%%", m.session.Accuracy())),
		m.renderStat("WPM", fmt.Sprintf("%.1f", m.session.WPM(m.elapsed()))),
	)

	var body strings.Builder
	body.WriteString(title)
	body.WriteString("\n")
	body.WriteString(statusLine)
	body.WriteString("\n\n")

	if m.state == stateFinished {
		body.WriteString(m.renderResults(contentWidth))
		body.WriteString("\n\n")
	} else {
		body.WriteString(statsRow)
		body.WriteString("\n\n")
		body.WriteString(promptBox)
		body.WriteString("\n\n")
	}

	body.WriteString(m.help.ShortHelpView(m.activeBindings()))

	content := lipgloss.NewStyle().
		Padding(1, 1).
		Width(contentWidth).
		Render(body.String())

	if m.height > 0 {
		return lipgloss.Place(width, m.height, lipgloss.Center, lipgloss.Center, content)
	}

	return content
}

func (m *model) resetSession() {
	prompt := buildPrompt(shuffleWords(m.words, m.rng))
	m.session = newTypingSession(prompt)
	m.state = stateReady
	m.startedAt = time.Time{}
	m.remaining = testDuration
}

func (m model) activeBindings() []key.Binding {
	if m.state == stateFinished {
		return []key.Binding{m.keys.restart, m.keys.quit}
	}

	return []key.Binding{m.keys.quit}
}

func (m model) statusCopy() string {
	switch m.state {
	case stateReady:
		return "Start typing to begin the 30 second test."
	case stateRunning:
		return "Type through the prompt. Backspace edits text, but raw mistakes still count."
	case stateFinished:
		return "Time is up. Review your results or press r for a fresh run."
	default:
		return ""
	}
}

func (m model) renderPrompt(width int) string {
	prompt := m.session.promptRunes
	if len(prompt) == 0 {
		return "No prompt available."
	}

	cursor := len(m.session.typed)
	start := maxInt(0, cursor-40)
	for start > 0 && prompt[start-1] != ' ' {
		start--
	}

	end := minInt(len(prompt), start+240)
	for end < len(prompt) && prompt[end] != ' ' {
		end++
	}

	correctStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#22C55E"))
	incorrectStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#F97316"))
	currentStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#FACC15")).
		Foreground(lipgloss.Color("#0F172A")).
		Bold(true)
	upcomingStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#CBD5E1"))
	fadedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#64748B"))

	var b strings.Builder
	if start > 0 {
		b.WriteString(fadedStyle.Render("... "))
	}

	for i := start; i < end; i++ {
		chunk := string(prompt[i])

		switch {
		case i < len(m.session.typed):
			if m.session.typed[i] == prompt[i] {
				b.WriteString(correctStyle.Render(chunk))
			} else {
				b.WriteString(incorrectStyle.Render(chunk))
			}
		case i == len(m.session.typed):
			b.WriteString(currentStyle.Render(chunk))
		default:
			b.WriteString(upcomingStyle.Render(chunk))
		}
	}

	if end < len(prompt) {
		b.WriteString(fadedStyle.Render(" ..."))
	}

	return lipgloss.NewStyle().Width(width).Render(b.String())
}

func (m model) renderResults(width int) string {
	headline := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#F8FAFC")).
		Render("Session complete")

	results := lipgloss.JoinHorizontal(
		lipgloss.Top,
		m.renderStat("WPM", fmt.Sprintf("%.1f", m.session.WPM(testDuration))),
		m.renderStat("Chars", fmt.Sprintf("%d", m.session.charsTyped)),
		m.renderStat("Accuracy", fmt.Sprintf("%.1f%%", m.session.Accuracy())),
	)

	box := strings.Join([]string{
		headline,
		"",
		results,
		"",
		lipgloss.NewStyle().Foreground(lipgloss.Color("#94A3B8")).Render("Press r to restart or q to quit."),
	}, "\n")

	return lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(lipgloss.Color("#FACC15")).
		Padding(1, 2).
		Width(width).
		Render(box)
}

func (m model) renderStat(label string, value string) string {
	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#94A3B8"))
	valueStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#F8FAFC"))

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#334155")).
		Padding(0, 1).
		MarginRight(1).
		Render(labelStyle.Render(label) + "\n" + valueStyle.Render(value))
}

func (m model) elapsed() time.Duration {
	switch m.state {
	case stateRunning:
		if m.startedAt.IsZero() {
			return 0
		}

		return minDuration(time.Since(m.startedAt), testDuration)
	case stateFinished:
		return testDuration
	default:
		return 0
	}
}

func (m model) remainingSeconds() float64 {
	return math.Max(0, m.remaining.Seconds())
}

func tickCmd() tea.Cmd {
	return tea.Tick(tickInterval, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func minDuration(a time.Duration, b time.Duration) time.Duration {
	if a < b {
		return a
	}

	return b
}

func minInt(a int, b int) int {
	if a < b {
		return a
	}

	return b
}

func maxInt(a int, b int) int {
	if a > b {
		return a
	}

	return b
}
