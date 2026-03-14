package main

import (
	"strings"
	"testing"
)

func TestBuildPromptLinesUsesSingleLineChunks(t *testing.T) {
	prompt := strings.Join([]string{
		"one", "two", "three", "four", "five", "six",
		"seven", "eight", "nine", "ten", "eleven", "twelve",
		"thirteen", "fourteen",
	}, " ")

	lines := buildPromptLines(prompt)
	if len(lines) != 2 {
		t.Fatalf("len(lines) = %d, want 2", len(lines))
	}

	firstCount := len(strings.Fields(lines[0].text))
	secondCount := len(strings.Fields(lines[1].text))

	if firstCount != promptWordsPerLine {
		t.Fatalf("first line word count = %d, want %d", firstCount, promptWordsPerLine)
	}

	if secondCount != 2 {
		t.Fatalf("second line word count = %d, want 2", secondCount)
	}
}

func TestCurrentPromptLineAdvancesAfterLineEnd(t *testing.T) {
	prompt := strings.Join([]string{
		"one", "two", "three", "four", "five", "six",
		"seven", "eight", "nine", "ten", "eleven", "twelve",
		"thirteen", "fourteen",
	}, " ")

	lines := buildPromptLines(prompt)
	if len(lines) != 2 {
		t.Fatalf("len(lines) = %d, want 2", len(lines))
	}

	firstIndex := currentPromptLineIndex(lines, 0)
	if firstIndex != 0 {
		t.Fatalf("firstIndex = %d, want 0", firstIndex)
	}

	secondIndex := currentPromptLineIndex(lines, lines[0].end)
	if secondIndex != 1 {
		t.Fatalf("secondIndex = %d, want 1", secondIndex)
	}
}
