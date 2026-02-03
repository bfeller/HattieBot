package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

type Input struct {
	URL string `json:"url"`
}

type Output struct {
	Content string `json:"content"`
	Error   string `json:"error,omitempty"`
}

func main() {
	var input Input
	if err := json.NewDecoder(os.Stdin).Decode(&input); err != nil {
		output := Output{Error: fmt.Sprintf("failed to parse input: %v", err)}
		writeOutput(output)
		os.Exit(1)
	}

	if input.URL == "" {
		output := Output{Error: "url is required"}
		writeOutput(output)
		os.Exit(1)
	}

	// Ensure URL has protocol
	url := input.URL
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		url = "https://" + url
	}

	// Build Jina AI URL
	jinaURL := "https://r.jina.ai/" + url

	// Fetch content
	resp, err := http.Get(jinaURL)
	if err != nil {
		output := Output{Error: fmt.Sprintf("failed to fetch URL: %v", err)}
		writeOutput(output)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		output := Output{Error: fmt.Sprintf("HTTP error: %d", resp.StatusCode)}
		writeOutput(output)
		os.Exit(1)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		output := Output{Error: fmt.Sprintf("failed to read response: %v", err)}
		writeOutput(output)
		os.Exit(1)
	}

	output := Output{Content: string(body)}
	writeOutput(output)
}

func writeOutput(output Output) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	enc.Encode(output)
}
