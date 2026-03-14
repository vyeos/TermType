package main

import (
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"
)

const (
	testDuration    = 30 * time.Second
	tickInterval    = 100 * time.Millisecond
	promptWordCount = 200
)

type typingSession struct {
	prompt       string
	promptRunes  []rune
	typed        []rune
	charsTyped   int
	correctChars int
}

func loadWords(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read words file: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	words := make([]string, 0, len(lines))

	for _, line := range lines {
		word := strings.TrimSpace(line)
		if word != "" {
			words = append(words, word)
		}
	}

	if len(words) == 0 {
		return nil, fmt.Errorf("no words found in %s", path)
	}

	return words, nil
}

func shuffleWords(words []string, rng *rand.Rand) []string {
	// New array just to return the shuffled array
	shuffled := append([]string(nil), words...)

	if rng == nil {
		rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	}

	rng.Shuffle(len(shuffled), func(i, j int) {
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	})

	return shuffled
}

func buildPrompt(words []string) string {
	if len(words) == 0 {
		return ""
	}

	limit := min(len(words), promptWordCount)

	return strings.Join(words[:limit], " ")
}

func newTypingSession(prompt string) typingSession {
	return typingSession{
		prompt:      prompt,
		promptRunes: []rune(prompt),
	}
}

// (the below text   ) is a value that can be used by the function but but not actually passed by the user
func (s *typingSession) TypeRune(r rune) {
	cursor := len(s.typed)
	if cursor < len(s.promptRunes) && s.promptRunes[cursor] == r {
		s.correctChars++
	}

	s.charsTyped++
	s.typed = append(s.typed, r)
}

func (s *typingSession) Backspace() {
	if len(s.typed) == 0 {
		return
	}

	s.typed = s.typed[:len(s.typed)-1]
}

func (s typingSession) Accuracy() float64 {
	if s.charsTyped == 0 {
		return 0
	}

	return float64(s.correctChars) / float64(s.charsTyped) * 100
}

func (s typingSession) WPM(elapsed time.Duration) float64 {
	if elapsed <= 0 {
		return 0
	}

	// using 5.0 because WPM is calculated using 5. I created a func but had to remove it
	return float64(s.correctChars) / 5.0 / elapsed.Minutes()
}
