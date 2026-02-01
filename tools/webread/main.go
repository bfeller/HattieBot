package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

type WebReadInput struct {
	URL string `json:"url"`
}

type WebReadOutput struct {
	URL     string `json:"url"`
	Content string `json:"content"`
	Error   string `json:"error,omitempty"`
}

func main() {
	var input WebReadInput
	if err := json.NewDecoder(os.Stdin).Decode(&input); err != nil {
		output := WebReadOutput{Error: fmt.Sprintf("Invalid input: %v", err)}
		json.NewEncoder(os.Stdout).Encode(output)
		os.Exit(1)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	jinaURL := "https://r.jina.ai/" + input.URL

	resp, err := client.Get(jinaURL)
	if err != nil {
		output := WebReadOutput{
			URL:   input.URL,
			Error: fmt.Sprintf("Request failed: %v", err),
		}
		json.NewEncoder(os.Stdout).Encode(output)
		os.Exit(1)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		output := WebReadOutput{
			URL:   input.URL,
			Error: fmt.Sprintf("Read failed: %v", err),
		}
		json.NewEncoder(os.Stdout).Encode(output)
		os.Exit(1)
	}

	output := WebReadOutput{
		URL:     input.URL,
		Content: string(body),
	}
	json.NewEncoder(os.Stdout).Encode(output)
}
