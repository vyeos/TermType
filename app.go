package main

import (
	"fmt"
	"math/rand"
	"strings"
	"time"
	"unicode"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
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
	duration        time.Duration
	mode            testMode
	wordGoal        int
	customWordsText string
	customWords     []string
}

type settingsFocus int

const (
	settingsFocusMode settingsFocus = iota
	settingsFocusDuration
	settingsFocusWordGoal
	settingsFocusCustomWords
	settingsFocusSave
	settingsFocusCancel
)

type keyMap struct {
	quit     key.Binding
	restart  key.Binding
	settings key.Binding
}

type model struct {
	defaultWords        []string
	rng                 *rand.Rand
	session             typingSession
	settings            appSettings
	draftSettings       appSettings
	settingsOpen        bool
	settingsFocus       settingsFocus
	settingsInput       textinput.Model
	resumeAfterSettings bool
	pauseStartedAt      time.Time
	finishedElapsed     time.Duration
	state               screenState
	startedAt           time.Time
	remaining           time.Duration
	width               int
	height              int
	help                help.Model
	keys                keyMap
}

func newModel(words []string, rng *rand.Rand) model {
	settings := defaultSettings()
	input := newSettingsInput()

	m := model{
		defaultWords: append([]string(nil), words...),
		rng:          rng,
		settings:     settings,
		draftSettings: appSettings{
			duration:        settings.duration,
			mode:            settings.mode,
			wordGoal:        settings.wordGoal,
			customWordsText: settings.customWordsText,
			customWords:     append([]string(nil), settings.customWords...),
		},
		settingsInput: input,
		state:         stateReady,
		remaining:     settings.duration,
		help:          help.New(),
		keys: keyMap{
			quit: key.NewBinding(
				key.WithKeys("q", "ctrl+c"),
				key.WithHelp("q", "quit"),
			),
			restart: key.NewBinding(
				key.WithKeys("r"),
				key.WithHelp("r", "restart"),
			),
			settings: key.NewBinding(
				key.WithKeys("ctrl+p"),
				key.WithHelp("ctrl+p", "settings"),
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

func newSettingsInput() textinput.Model {
	input := textinput.New()
	input.Placeholder = "custom words separated by spaces, commas, or new lines"
	input.Prompt = ""
	input.Width = 44
	input.Blur()
	return input
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

		if m.settings.mode != modeTimed {
			return m, nil
		}

		if m.settingsOpen {
			return m, tickCmd()
		}

		elapsed := time.Time(msg).Sub(m.startedAt)
		if elapsed >= m.settings.duration {
			m.finishSession(m.settings.duration)
			return m, nil
		}

		m.remaining = m.settings.duration - elapsed
		return m, tickCmd()

	case tea.KeyMsg:
		if m.settingsOpen {
			if msg.Type == tea.KeyCtrlC {
				return m, tea.Quit
			}

			return m.updateSettings(msg)
		}

		if key.Matches(msg, m.keys.settings) {
			m.openSettings()
			return m, nil
		}

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

func (m model) updateSettings(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.closeSettings(false)
		return m, nil
	case tea.KeyTab, tea.KeyDown:
		m.moveSettingsFocus(1)
		return m, nil
	case tea.KeyShiftTab, tea.KeyUp:
		m.moveSettingsFocus(-1)
		return m, nil
	case tea.KeyLeft:
		if m.settingsFocus == settingsFocusMode {
			m.shiftMode(-1)
			return m, nil
		}
		if m.settingsFocus == settingsFocusDuration {
			m.shiftDuration(-1)
			return m, nil
		}
		if m.settingsFocus == settingsFocusWordGoal {
			m.shiftWordGoal(-1)
			return m, nil
		}
		if m.settingsFocus == settingsFocusCancel {
			m.settingsFocus = settingsFocusSave
			m.syncSettingsInputFocus()
		}
		return m, nil
	case tea.KeyRight:
		if m.settingsFocus == settingsFocusMode {
			m.shiftMode(1)
			return m, nil
		}
		if m.settingsFocus == settingsFocusDuration {
			m.shiftDuration(1)
			return m, nil
		}
		if m.settingsFocus == settingsFocusWordGoal {
			m.shiftWordGoal(1)
			return m, nil
		}
		if m.settingsFocus == settingsFocusSave {
			m.settingsFocus = settingsFocusCancel
			m.syncSettingsInputFocus()
		}
		return m, nil
	case tea.KeyEnter:
		switch m.settingsFocus {
		case settingsFocusSave:
			m.closeSettings(true)
			return m, nil
		case settingsFocusCancel:
			m.closeSettings(false)
			return m, nil
		}
	}

	if m.settingsFocus == settingsFocusCustomWords {
		var cmd tea.Cmd
		m.settingsInput, cmd = m.settingsInput.Update(msg)
		m.draftSettings.customWordsText = m.settingsInput.Value()
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

	if m.state == stateFinished {
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

	if m.settingsOpen {
		return m.renderSettingsModal(width)
	}

	if m.height > 0 {
		return lipgloss.Place(width, m.height, lipgloss.Center, lipgloss.Center, content)
	}

	return content
}

func (m *model) openSettings() {
	m.settingsOpen = true
	m.draftSettings = appSettings{
		duration:        m.settings.duration,
		mode:            m.settings.mode,
		wordGoal:        m.settings.wordGoal,
		customWordsText: m.settings.customWordsText,
		customWords:     append([]string(nil), m.settings.customWords...),
	}
	m.settingsFocus = settingsFocusMode
	m.settingsInput.SetValue(m.settings.customWordsText)
	m.settingsInput.CursorEnd()
	m.resumeAfterSettings = m.state == stateRunning
	if m.resumeAfterSettings {
		m.pauseStartedAt = time.Now()
	}
	m.syncSettingsInputFocus()
}

func (m *model) closeSettings(save bool) {
	if save {
		m.settings = appSettings{
			duration:        m.draftSettings.duration,
			mode:            m.draftSettings.mode,
			wordGoal:        m.draftSettings.wordGoal,
			customWordsText: m.settingsInput.Value(),
		}
		m.settings.customWords = append([]string(nil), parseCustomWords(m.settings.customWordsText)...)
		m.settingsOpen = false
		m.resumeAfterSettings = false
		m.pauseStartedAt = time.Time{}
		m.resetSession()
		return
	}

	m.settingsOpen = false
	if m.resumeAfterSettings {
		m.startedAt = m.startedAt.Add(time.Since(m.pauseStartedAt))
		m.pauseStartedAt = time.Time{}
		m.resumeAfterSettings = false
	}
}

func (m *model) moveSettingsFocus(delta int) {
	total := 6
	next := (int(m.settingsFocus) + delta + total) % total
	m.settingsFocus = settingsFocus(next)
	m.syncSettingsInputFocus()
}

func (m *model) syncSettingsInputFocus() {
	if m.settingsFocus == settingsFocusCustomWords {
		m.settingsInput.Focus()
		return
	}

	m.settingsInput.Blur()
}

func (m *model) shiftDuration(delta int) {
	index := 0
	for i, option := range durationOptions {
		if option == m.draftSettings.duration {
			index = i
			break
		}
	}

	index = (index + delta + len(durationOptions)) % len(durationOptions)
	m.draftSettings.duration = durationOptions[index]
}

func (m *model) shiftMode(delta int) {
	modes := []testMode{modeTimed, modeWordGoal}
	index := 0
	for i, mode := range modes {
		if mode == m.draftSettings.mode {
			index = i
			break
		}
	}

	index = (index + delta + len(modes)) % len(modes)
	m.draftSettings.mode = modes[index]
}

func (m *model) shiftWordGoal(delta int) {
	index := 0
	for i, goal := range wordGoalOptions {
		if goal == m.draftSettings.wordGoal {
			index = i
			break
		}
	}

	index = (index + delta + len(wordGoalOptions)) % len(wordGoalOptions)
	m.draftSettings.wordGoal = wordGoalOptions[index]
}

func (m *model) resetSession() {
	prompt := buildPrompt(shuffleWords(m.currentWords(), m.rng))
	m.session = newTypingSession(prompt)
	m.state = stateReady
	m.startedAt = time.Time{}
	m.remaining = m.settings.duration
	m.finishedElapsed = 0
}

func (m model) currentWords() []string {
	if len(m.settings.customWords) > 0 {
		return append([]string(nil), m.settings.customWords...)
	}

	return append([]string(nil), m.defaultWords...)
}

func (m *model) finishSession(elapsed time.Duration) {
	m.remaining = 0
	m.finishedElapsed = elapsed
	m.state = stateFinished
}

func (m model) activeBindings() []key.Binding {
	if m.state == stateFinished {
		return []key.Binding{m.keys.restart, m.keys.settings, m.keys.quit}
	}

	return []key.Binding{m.keys.settings, m.keys.quit}
}

func (m model) statusCopy() string {
	switch m.state {
	case stateReady:
		if m.settings.mode == modeWordGoal {
			return fmt.Sprintf("Start typing to complete the %d-word goal. Press Ctrl+P for settings.", m.settings.wordGoal)
		}
		return fmt.Sprintf("Start typing to begin the %s test. Press Ctrl+P for settings.", formatDurationLabel(m.settings.duration))
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

	return strings.Join([]string{centeredCurrent, centeredNext}, "\n\n")
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
		lipgloss.NewStyle().Foreground(lipgloss.Color("#94A3B8")).Render("Press r to restart, ctrl+p for settings, or q to quit."),
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

func (m model) renderSettingsModal(screenWidth int) string {
	width := minInt(76, maxInt(56, screenWidth-8))

	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#F8FAFC")).
		Render("Settings")

	helpText := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#94A3B8")).
		Render("Tab moves focus. Left and right change the selected control. Saving applies settings to a fresh session.")

	modeLabel := m.renderSettingsLabel("Mode", m.settingsFocus == settingsFocusMode)
	durationLabel := m.renderSettingsLabel("Time", m.settingsFocus == settingsFocusDuration)
	wordGoalLabel := m.renderSettingsLabel("Word Goal", m.settingsFocus == settingsFocusWordGoal)
	customLabel := m.renderSettingsLabel("Custom words", m.settingsFocus == settingsFocusCustomWords)
	saveButton := m.renderButton("Save", m.settingsFocus == settingsFocusSave)
	cancelButton := m.renderButton("Cancel", m.settingsFocus == settingsFocusCancel)

	modeRow := lipgloss.JoinHorizontal(lipgloss.Left, m.renderModeOptions()...)
	durationRow := lipgloss.JoinHorizontal(lipgloss.Left, m.renderDurationOptions()...)
	wordGoalRow := lipgloss.JoinHorizontal(lipgloss.Left, m.renderWordGoalOptions()...)
	actions := lipgloss.JoinHorizontal(lipgloss.Left, saveButton, cancelButton)

	body := strings.Join([]string{
		title,
		helpText,
		"",
		modeLabel,
		modeRow,
		"",
		durationLabel,
		durationRow,
		"",
		wordGoalLabel,
		wordGoalRow,
		"",
		customLabel,
		m.renderSettingsInput(width - 8),
		"",
		actions,
	}, "\n")

	modal := lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(lipgloss.Color("#38BDF8")).
		Padding(1, 2).
		Width(width).
		Render(body)

	if m.height > 0 {
		return lipgloss.Place(screenWidth, m.height, lipgloss.Center, lipgloss.Center, modal)
	}

	return modal
}

func (m model) renderSettingsLabel(label string, focused bool) string {
	style := lipgloss.NewStyle().Foreground(lipgloss.Color("#94A3B8"))
	if focused {
		style = style.Bold(true).Foreground(lipgloss.Color("#F8FAFC"))
	}
	return style.Render(label)
}

func (m model) renderModeOptions() []string {
	modes := []struct {
		mode  testMode
		label string
	}{
		{mode: modeTimed, label: "Timed"},
		{mode: modeWordGoal, label: "Word Goal"},
	}

	options := make([]string, 0, len(modes))
	for _, option := range modes {
		selected := option.mode == m.draftSettings.mode
		style := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#334155")).
			Padding(0, 1).
			MarginRight(1)
		if selected {
			style = style.
				Bold(true).
				BorderForeground(lipgloss.Color("#38BDF8")).
				Foreground(lipgloss.Color("#F8FAFC"))
		}
		options = append(options, style.Render(option.label))
	}
	return options
}

func (m model) renderDurationOptions() []string {
	options := make([]string, 0, len(durationOptions))
	for _, option := range durationOptions {
		selected := option == m.draftSettings.duration
		style := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#334155")).
			Padding(0, 1).
			MarginRight(1)
		if selected {
			style = style.
				Bold(true).
				BorderForeground(lipgloss.Color("#38BDF8")).
				Foreground(lipgloss.Color("#F8FAFC"))
		}
		options = append(options, style.Render(formatDurationLabel(option)))
	}
	return options
}

func (m model) renderWordGoalOptions() []string {
	options := make([]string, 0, len(wordGoalOptions))
	for _, goal := range wordGoalOptions {
		selected := goal == m.draftSettings.wordGoal
		style := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#334155")).
			Padding(0, 1).
			MarginRight(1)
		if selected {
			style = style.
				Bold(true).
				BorderForeground(lipgloss.Color("#38BDF8")).
				Foreground(lipgloss.Color("#F8FAFC"))
		}
		options = append(options, style.Render(fmt.Sprintf("%d words", goal)))
	}
	return options
}

func (m model) renderSettingsInput(width int) string {
	input := m.settingsInput.View()
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#334155")).
		Padding(0, 1).
		Width(width)

	if m.settingsFocus == settingsFocusCustomWords {
		style = style.BorderForeground(lipgloss.Color("#38BDF8"))
	}

	return style.Render(input)
}

func (m model) renderButton(label string, focused bool) string {
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#334155")).
		Padding(0, 2).
		MarginRight(1)
	if focused {
		style = style.
			Bold(true).
			BorderForeground(lipgloss.Color("#38BDF8")).
			Foreground(lipgloss.Color("#F8FAFC"))
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

func currentPromptLine(lines []promptLine, cursor int) promptLine {
	return lines[currentPromptLineIndex(lines, cursor)]
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
