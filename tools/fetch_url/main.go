package main

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
)

func main() {
	var input struct {
		URL string `json:"url"`
	}
	
	result := make(map[string]string)
	
	if err := json.NewDecoder(os.Stdin).Decode(&input); err != nil {
		result["error"] = err.Error()
		json.NewEncoder(os.Stdout).Encode(result)
		return
	}

	if input.URL == "" {
		result["error"] = "url is required"
		json.NewEncoder(os.Stdout).Encode(result)
		return
	}

	url := input.URL
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		url = "https://" + url
	}

	jinaURL := "https://r.jina.ai/" + url
	resp, err := http.Get(jinaURL)
	if err != nil {
		result["error"] = err.Error()
		json.NewEncoder(os.Stdout).Encode(result)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		result["error"] = "HTTP error: " + resp.Status
		json.NewEncoder(os.Stdout).Encode(result)
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		result["error"] = err.Error()
		json.NewEncoder(os.Stdout).Encode(result)
		return
	}

	result["content"] = string(body)
	json.NewEncoder(os.Stdout).Encode(result)
}
