package main

import (
	"math"
	"math/rand"
	"strings"
	"testing"
	"time"
)

func TestLoadWordsBuildsNonEmptyPrompt(t *testing.T) {
	words, err := loadWords()
	if err != nil {
		t.Fatalf("loadWords returned error: %v", err)
	}

	if len(words) == 0 {
		t.Fatal("expected at least one word")
	}

	shuffled := shuffleWords(words, rand.New(rand.NewSource(7)))
	prompt := buildPrompt(shuffled)
	if strings.TrimSpace(prompt) == "" {
		t.Fatal("expected non-empty prompt")
	}
}

func TestBuildPromptRepeatsShortLists(t *testing.T) {
	prompt := buildPrompt([]string{"alpha", "beta"})
	words := strings.Fields(prompt)

	if len(words) != promptWordCount {
		t.Fatalf("len(words) = %d, want %d", len(words), promptWordCount)
	}

	if words[0] != "alpha" || words[1] != "beta" || words[2] != "alpha" {
		t.Fatalf("unexpected repeated sequence: %v", words[:3])
	}
}

func TestWordGoalCursor(t *testing.T) {
	prompt := "alpha beta gamma delta"
	if got := wordGoalCursor(prompt, 2); got != len([]rune("alpha beta")) {
		t.Fatalf("wordGoalCursor = %d, want %d", got, len([]rune("alpha beta")))
	}
}

func TestCompletedPromptWords(t *testing.T) {
	prompt := "alpha beta gamma delta"
	if got := completedPromptWords(prompt, len([]rune("alpha beta"))); got != 2 {
		t.Fatalf("completedPromptWords = %d, want 2", got)
	}

	if got := completedPromptWords(prompt, len([]rune("alpha be"))); got != 1 {
		t.Fatalf("completedPromptWords with partial second word = %d, want 1", got)
	}
}

func TestTypingSessionFullyCorrectInput(t *testing.T) {
	session := newTypingSession("cat")
	for _, r := range "cat" {
		session.TypeRune(r)
	}

	if session.charsTyped != 3 {
		t.Fatalf("charsTyped = %d, want 3", session.charsTyped)
	}

	if session.correctChars != 3 {
		t.Fatalf("correctChars = %d, want 3", session.correctChars)
	}

	if session.Accuracy() != 100 {
		t.Fatalf("accuracy = %.2f, want 100", session.Accuracy())
	}
}

func TestTypingSessionIncorrectCharacters(t *testing.T) {
	session := newTypingSession("cat")
	for _, r := range "car" {
		session.TypeRune(r)
	}

	if session.charsTyped != 3 {
		t.Fatalf("charsTyped = %d, want 3", session.charsTyped)
	}

	if session.correctChars != 2 {
		t.Fatalf("correctChars = %d, want 2", session.correctChars)
	}

	assertNear(t, session.Accuracy(), 66.6667, 0.01)
}

func TestTypingSessionBackspaceKeepsRawErrors(t *testing.T) {
	session := newTypingSession("cat")

	session.TypeRune('x')
	session.Backspace()
	session.TypeRune('c')
	session.TypeRune('a')
	session.TypeRune('t')

	if session.charsTyped != 4 {
		t.Fatalf("charsTyped = %d, want 4", session.charsTyped)
	}

	if session.correctChars != 3 {
		t.Fatalf("correctChars = %d, want 3", session.correctChars)
	}

	if string(session.typed) != "cat" {
		t.Fatalf("typed = %q, want %q", string(session.typed), "cat")
	}

	assertNear(t, session.Accuracy(), 75, 0.001)
}

func TestTypingSessionMixedInput(t *testing.T) {
	session := newTypingSession("cat dog")

	for _, r := range []rune{'c', 'a', 'x'} {
		session.TypeRune(r)
	}

	session.Backspace()

	for _, r := range []rune{'t', ' ', 'd', 'o', 'z'} {
		session.TypeRune(r)
	}

	if session.charsTyped != 8 {
		t.Fatalf("charsTyped = %d, want 8", session.charsTyped)
	}

	if session.correctChars != 6 {
		t.Fatalf("correctChars = %d, want 6", session.correctChars)
	}

	if string(session.typed) != "cat doz" {
		t.Fatalf("typed = %q, want %q", string(session.typed), "cat doz")
	}

	assertNear(t, session.Accuracy(), 75, 0.001)
	assertNear(t, session.WPM(30*time.Second), 2.4, 0.001)
}

func assertNear(t *testing.T, got float64, want float64, tolerance float64) {
	t.Helper()

	if math.Abs(got-want) > tolerance {
		t.Fatalf("got %.4f, want %.4f (+/- %.4f)", got, want, tolerance)
	}
}
