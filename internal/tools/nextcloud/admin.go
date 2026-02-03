package nextcloud

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/hattiebot/hattiebot/internal/config"
)

// RequestNextcloudOCS executes a Nextcloud OCS API request aka "Provisioning API".
// method: GET, POST, PUT, DELETE
// endpoint: e.g. "/cloud/users" (will be prefixed with /ocs/v1.php or /ocs/v2.php)
// params: map of string params. For GET they are query, for POST they are form-encoded body.
func RequestNextcloudOCS(cfg *config.Config, method, endpoint string, params map[string]string) (string, error) {
	if cfg.NextcloudURL == "" || cfg.NextcloudBotUser == "" || cfg.NextcloudBotAppPassword == "" {
		return "", fmt.Errorf("nextcloud credentials not configured")
	}

	baseURL := strings.TrimRight(cfg.NextcloudURL, "/")
	// Default to v1.php for broad compatibility, usually it supports both.
	// Users might pass /cloud/users.
	if !strings.HasPrefix(endpoint, "/") {
		endpoint = "/" + endpoint
	}
	apiURL := fmt.Sprintf("%s/ocs/v1.php%s", baseURL, endpoint)

	data := url.Values{}
	for k, v := range params {
		data.Set(k, v)
	}

	var body io.Reader
	if method == "GET" || method == "DELETE" {
		if len(params) > 0 {
			apiURL += "?" + data.Encode()
		}
	} else {
		body = strings.NewReader(data.Encode())
	}

	req, err := http.NewRequest(method, apiURL, body)
	if err != nil {
		return "", err
	}
	req.SetBasicAuth(cfg.NextcloudBotUser, cfg.NextcloudBotAppPassword)
	req.Header.Set("OCS-APIRequest", "true")
	req.Header.Set("Accept", "application/json")
	if method == "POST" || method == "PUT" {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	
	// Basic status check
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
	}

	return string(respBody), nil
}
