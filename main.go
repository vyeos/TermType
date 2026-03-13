package main

import (
	"fmt"
	"math/rand"
	"os"
	"strings"
)

func shuffleWords() {
	dat, err := os.ReadFile("500_common_words.txt")
	if err != nil {
		fmt.Printf("error: %v\n", err)
		return
	}

	words := strings.Split(strings.TrimSpace(string(dat)), "\n")
	rand.Shuffle(len(words), func(i, j int) {
		words[i], words[j] = words[j], words[i]
	})

	for _, word := range words {
		fmt.Println(word)
	}
}

func main() {
	shuffleWords()
}
