package main

import (
	"fmt"
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

const promptWordsPerLine = 12

var durationOptions = []time.Duration{
	15 * time.Second,
	30 * time.Second,
	60 * time.Second,
	120 * time.Second,
}

var wordGoalOptions = []int{10, 25, 50, 100}

type tickMsg time.Time

type promptLine struct {
	start int
	end   int
	text  string
}

type testMode int

const (
	modeTimed testMode = iota
	modeWordGoal
)

type appSettings struct {
	duration time.Duration
	mode     testMode
	wordGoal int
}

type keyMap struct {
	quit    key.Binding
	restart key.Binding
}

type model struct {
	defaultWords    []string
	rng             *rand.Rand
	session         typingSession
	settings        appSettings
	finishedElapsed time.Duration
	state           screenState
	startedAt       time.Time
	remaining       time.Duration
	width           int
	height          int
	help            help.Model
	keys            keyMap
}

func newModel(words []string, rng *rand.Rand) model {
	settings := defaultSettings()

	m := model{
		defaultWords:    append([]string(nil), words...),
		rng:             rng,
		settings:        settings,
		state:           stateReady,
		remaining:       settings.duration,
		help:            help.New(),
		finishedElapsed: 0,
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

func defaultSettings() appSettings {
	return appSettings{
		duration: defaultTestDuration,
		mode:     modeTimed,
		wordGoal: 25,
	}
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
		if m.state != stateRunning || m.settings.mode != modeTimed {
			return m, nil
		}

		elapsed := time.Time(msg).Sub(m.startedAt)
		if elapsed >= m.settings.duration {
			m.finishSession(m.settings.duration)
			return m, nil
		}

		m.remaining = m.settings.duration - elapsed
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

		if m.state == stateReady {
			return m.updateReady(msg)
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
			m.remaining = m.settings.duration
			if m.settings.mode == modeTimed {
				cmd = tickCmd()
			}
		}

		m.session.TypeRune(msg.Runes[0])
		if m.settings.mode == modeWordGoal && len(m.session.typed) >= wordGoalCursor(m.session.prompt, m.settings.wordGoal) {
			m.finishSession(time.Since(m.startedAt))
			return m, nil
		}

		return m, cmd
	}

	return m, nil
}

func (m model) updateReady(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyTab:
		m.toggleMode()
		return m, nil
	case tea.KeyUp, tea.KeyDown:
		return m, nil
	case tea.KeyLeft:
		m.shiftActiveOption(-1)
		return m, nil
	case tea.KeyRight:
		m.shiftActiveOption(1)
		return m, nil
	}

	switch msg.Type {
	case tea.KeyBackspace, tea.KeyCtrlH:
		return m, nil
	}

	if len(msg.Runes) != 1 || !unicode.IsPrint(msg.Runes[0]) {
		return m, nil
	}

	cmd := tea.Cmd(nil)
	m.state = stateRunning
	m.startedAt = time.Now()
	m.remaining = m.settings.duration
	if m.settings.mode == modeTimed {
		cmd = tickCmd()
	}

	m.session.TypeRune(msg.Runes[0])
	if m.settings.mode == modeWordGoal && len(m.session.typed) >= wordGoalCursor(m.session.prompt, m.settings.wordGoal) {
		m.finishSession(time.Since(m.startedAt))
		return m, nil
	}

	return m, cmd
}

func (m model) View() string {
	width := m.width
	if width == 0 {
		width = 100
	}

	contentWidth := maxInt(56, width-6)
	if width < 62 {
		contentWidth = maxInt(24, width-2)
	}

	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#F8FAFC")).
		Render("GoTap")

	subtitle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#94A3B8"))

	statusLine := subtitle.Render(m.statusCopy())
	runStatBox := m.renderRunStat()
	prompt := m.renderPrompt(contentWidth - 4)
	promptBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#334155")).
		Padding(1, 2).
		Width(contentWidth).
		Render(prompt)

	var body strings.Builder
	body.WriteString(title)
	body.WriteString("\n")
	body.WriteString(statusLine)
	body.WriteString("\n\n")

	if m.state == stateReady {
		body.WriteString(m.renderStartOptions(contentWidth))
		body.WriteString("\n\n")
		body.WriteString(promptBox)
		body.WriteString("\n\n")
	} else if m.state == stateFinished {
		body.WriteString(m.renderResults(contentWidth))
		body.WriteString("\n\n")
	} else {
		body.WriteString(runStatBox)
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

func (m *model) toggleMode() {
	if m.settings.mode == modeTimed {
		m.settings.mode = modeWordGoal
	} else {
		m.settings.mode = modeTimed
	}

	m.resetSession()
}

func (m *model) shiftActiveOption(delta int) {
	if m.settings.mode == modeTimed {
		m.shiftDuration(delta)
		return
	}

	m.shiftWordGoal(delta)
}

func (m *model) shiftDuration(delta int) {
	index := 0
	for i, option := range durationOptions {
		if option == m.settings.duration {
			index = i
			break
		}
	}

	index = (index + delta + len(durationOptions)) % len(durationOptions)
	m.settings.duration = durationOptions[index]
	m.resetSession()
}

func (m *model) shiftWordGoal(delta int) {
	index := 0
	for i, goal := range wordGoalOptions {
		if goal == m.settings.wordGoal {
			index = i
			break
		}
	}

	index = (index + delta + len(wordGoalOptions)) % len(wordGoalOptions)
	m.settings.wordGoal = wordGoalOptions[index]
	m.resetSession()
}

func (m *model) resetSession() {
	prompt := buildPrompt(shuffleWords(m.defaultWords, m.rng))
	m.session = newTypingSession(prompt)
	m.state = stateReady
	m.startedAt = time.Time{}
	m.remaining = m.settings.duration
	m.finishedElapsed = 0
}

func (m *model) finishSession(elapsed time.Duration) {
	m.remaining = 0
	m.finishedElapsed = elapsed
	m.state = stateFinished
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
		if m.settings.mode == modeWordGoal {
			return fmt.Sprintf("Word Goal mode: %d words. Tab switches mode and left/right change the goal.", m.settings.wordGoal)
		}
		return fmt.Sprintf("Timed mode: %s. Tab switches mode and left/right change the timer.", formatDurationLabel(m.settings.duration))
	case stateRunning:
		if m.settings.mode == modeWordGoal {
			return fmt.Sprintf("Reach %d words to finish. Completed lines slide away; raw mistakes still count.", m.settings.wordGoal)
		}
		return "Type through the line. Completed lines slide away; raw mistakes still count."
	case stateFinished:
		if m.settings.mode == modeWordGoal {
			return "Word goal complete. Review your results or press r for a fresh run."
		}
		return "Time is up. Review your results or press r for a fresh run."
	default:
		return ""
	}
}

func (m model) renderPrompt(width int) string {
	lines := buildPromptLines(m.session.prompt)
	if len(lines) == 0 {
		return "No prompt available."
	}

	cursor := len(m.session.typed)
	lineIndex := currentPromptLineIndex(lines, cursor)
	prompt := m.session.promptRunes

	correctStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#22C55E"))
	incorrectStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#F97316"))
	currentStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#FACC15")).
		Foreground(lipgloss.Color("#0F172A")).
		Bold(true)
	upcomingStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#CBD5E1"))

	var currentLine strings.Builder
	line := lines[lineIndex]
	for i := line.start; i < line.end; i++ {
		chunk := string(prompt[i])

		switch {
		case i < len(m.session.typed):
			if m.session.typed[i] == prompt[i] {
				currentLine.WriteString(correctStyle.Render(chunk))
			} else {
				currentLine.WriteString(incorrectStyle.Render(chunk))
			}
		case i == len(m.session.typed):
			currentLine.WriteString(currentStyle.Render(chunk))
		default:
			currentLine.WriteString(upcomingStyle.Render(chunk))
		}
	}

	nextLine := ""
	if lineIndex+1 < len(lines) {
		nextLine = upcomingStyle.Render(lines[lineIndex+1].text)
	}

	centeredCurrent := lipgloss.PlaceHorizontal(width, lipgloss.Center, currentLine.String())
	centeredNext := lipgloss.PlaceHorizontal(width, lipgloss.Center, nextLine)

	return strings.Join([]string{centeredCurrent, centeredNext}, "\n")
}

func (m model) renderResults(width int) string {
	headline := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#F8FAFC")).
		Render(m.resultsHeadline())

	elapsed := m.finishedElapsed
	if elapsed <= 0 {
		elapsed = m.settings.duration
	}

	results := lipgloss.JoinHorizontal(
		lipgloss.Top,
		m.renderStat("WPM", fmt.Sprintf("%.1f", m.session.WPM(elapsed))),
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

func (m model) renderStartOptions(width int) string {
	optionLabel := "Time"
	optionRow := m.renderDurationOptions()
	if m.settings.mode == modeWordGoal {
		optionLabel = "Word Goal"
		optionRow = m.renderWordGoalOptions()
	}

	modeRow := lipgloss.JoinHorizontal(
		lipgloss.Center,
		m.renderOptionLabel("Mode"),
		" ",
		lipgloss.JoinHorizontal(lipgloss.Left, m.renderModeOptions()...),
	)
	optionLine := lipgloss.JoinHorizontal(
		lipgloss.Center,
		m.renderOptionLabel(optionLabel),
		" ",
		lipgloss.JoinHorizontal(lipgloss.Left, optionRow...),
	)

	return lipgloss.NewStyle().
		Width(width).
		Render(strings.Join([]string{modeRow, optionLine}, "\n"))
}

func (m model) renderOptionLabel(label string) string {
	return lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#F8FAFC")).
		Render(label)
}

func (m model) renderModeOptions() []string {
	return []string{
		m.renderChoiceChip("Timed", m.settings.mode == modeTimed),
		m.renderChoiceChip("Word Goal", m.settings.mode == modeWordGoal),
	}
}

func (m model) renderDurationOptions() []string {
	options := make([]string, 0, len(durationOptions))
	for _, option := range durationOptions {
		options = append(options, m.renderChoiceChip(formatDurationLabel(option), option == m.settings.duration))
	}
	return options
}

func (m model) renderWordGoalOptions() []string {
	options := make([]string, 0, len(wordGoalOptions))
	for _, goal := range wordGoalOptions {
		options = append(options, m.renderChoiceChip(fmt.Sprintf("%d words", goal), goal == m.settings.wordGoal))
	}
	return options
}

func (m model) renderChoiceChip(label string, selected bool) string {
	style := lipgloss.NewStyle().
		Border(lipgloss.Border{
			Left:  "│",
			Right: "│",
		}).
		BorderForeground(lipgloss.Color("#334155")).
		Padding(0, 1).
		MarginRight(1).
		Foreground(lipgloss.Color("#CBD5E1"))

	if selected {
		style = style.
			Bold(true).
			Foreground(lipgloss.Color("#F8FAFC")).
			BorderForeground(lipgloss.Color("#38BDF8")).
			Background(lipgloss.Color("#0F172A"))
	}

	return style.Render(label)
}

func (m model) remainingSeconds() float64 {
	if m.remaining < 0 {
		return 0
	}

	return m.remaining.Seconds()
}

func (m model) renderRunStat() string {
	if m.settings.mode == modeWordGoal {
		completed := completedPromptWords(m.session.prompt, len(m.session.typed))
		if completed > m.settings.wordGoal {
			completed = m.settings.wordGoal
		}
		return m.renderStat("Words", fmt.Sprintf("%d/%d", completed, m.settings.wordGoal))
	}

	return m.renderStat("Time", fmt.Sprintf("%.1fs", m.remainingSeconds()))
}

func tickCmd() tea.Cmd {
	return tea.Tick(tickInterval, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
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

func formatDurationLabel(duration time.Duration) string {
	return fmt.Sprintf("%ds", int(duration.Seconds()))
}

func (m model) resultsHeadline() string {
	if m.settings.mode == modeWordGoal {
		return "Word goal complete"
	}

	return "Session complete"
}

func buildPromptLines(prompt string) []promptLine {
	words := strings.Fields(prompt)
	if len(words) == 0 {
		return nil
	}

	lines := make([]promptLine, 0, (len(words)+promptWordsPerLine-1)/promptWordsPerLine)
	cursor := 0

	for i := 0; i < len(words); i += promptWordsPerLine {
		endWord := minInt(len(words), i+promptWordsPerLine)
		lineText := strings.Join(words[i:endWord], " ")
		lineLength := len([]rune(lineText))
		lineEnd := cursor + lineLength

		if endWord < len(words) {
			lineEnd++
		}

		lines = append(lines, promptLine{
			start: cursor,
			end:   lineEnd,
			text:  lineText,
		})
		cursor = lineEnd
	}

	return lines
}

func currentPromptLineIndex(lines []promptLine, cursor int) int {
	if len(lines) == 0 {
		return 0
	}

	for i, line := range lines {
		if cursor < line.end {
			return i
		}
	}

	return len(lines) - 1
}
