package main

import (
	"ai-cli/ai"
	"fmt"
	"log"
	"os"
	"strings"
)

const (
	aiWorkerURL = "http://localhost:8000"
)

func main() {
	if len(os.Args) < 3 {
		log.Fatal("Usage: ai ask \"your prompt\"")
		return
	}

	command := os.Args[1]
	prompt := strings.Join(os.Args[2:], " ")
	client := ai.Client{BaseURL: aiWorkerURL}

	switch command {
	case "ask":
		resp, err := client.Generate(prompt)
		if err != nil {
			log.Fatal("error:", err)
			return
		}
		fmt.Println(resp)

	default:
		log.Fatal("Unknown command")
	}
}
