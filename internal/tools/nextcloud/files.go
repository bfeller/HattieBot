package nextcloud

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/hattiebot/hattiebot/internal/config"
)

// ListNextcloudFiles uses WebDAV PROPFIND to list files.
func ListNextcloudFiles(cfg *config.Config, path string) (string, error) {
	if cfg.NextcloudURL == "" || cfg.NextcloudBotUser == "" || cfg.NextcloudBotAppPassword == "" {
		return "", fmt.Errorf("nextcloud credentials not configured")
	}

    // WebDAV base: /remote.php/dav/files/USER/PATH
    baseURL := strings.TrimRight(cfg.NextcloudURL, "/")
    user := cfg.NextcloudBotUser
    path = strings.TrimLeft(path, "/")
    
    davURL := fmt.Sprintf("%s/remote.php/dav/files/%s/%s", baseURL, user, path)

    req, _ := http.NewRequest("PROPFIND", davURL, nil)
    req.SetBasicAuth(user, cfg.NextcloudBotAppPassword)
    req.Header.Set("Depth", "1") // Immediate children

    client := &http.Client{Timeout: 30 * time.Second}
    resp, err := client.Do(req)
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()
    
    body, _ := io.ReadAll(resp.Body)
    if resp.StatusCode >= 400 {
        return "", fmt.Errorf("WebDAV error %d: %s", resp.StatusCode, string(body))
    }

    // Parse minimal XML manually or simple strings extraction to keep it lightweight for LLM?
    // A proper XML struct is better but verbose.
    // For now, let's return the raw XML or a simplified list.
    // The LLM can handle XML, but maybe cleaner to parse.
    // Let's assume LLM can handle the XML MultiStatus response for now, it's standard.
    // Or we provide a simple parser. To save context, let's simple parse.
    
    return parseWebDavList(body)
}

// WriteNextcloudFile uploads content to a file path using WebDAV.
func WriteNextcloudFile(cfg *config.Config, path, content string) error {
    if cfg.NextcloudURL == "" || cfg.NextcloudBotUser == "" || cfg.NextcloudBotAppPassword == "" {
		return fmt.Errorf("nextcloud credentials not configured")
	}

    baseURL := strings.TrimRight(cfg.NextcloudURL, "/")
    if !strings.HasPrefix(path, "/") {
        path = "/" + path
    }
    // WebDAV endpoint
    davURL := fmt.Sprintf("%s/remote.php/dav/files/%s%s", baseURL, cfg.NextcloudBotUser, path)
    
    req, _ := http.NewRequest("PUT", davURL, strings.NewReader(content))
    req.SetBasicAuth(cfg.NextcloudBotUser, cfg.NextcloudBotAppPassword)
    req.Header.Set("Content-Type", "text/plain")
    
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
    
    if resp.StatusCode >= 300 {
        body, _ := io.ReadAll(resp.Body)
        return fmt.Errorf("upload failed (%d): %s", resp.StatusCode, string(body))
    }
    return nil
}

// ShareNextcloudFile shares a file with a user (e.g. admin).
func ShareNextcloudFile(cfg *config.Config, path, shareWith string) error {
	params := map[string]string{
		"path":         path,
		"shareType":    "0",
		"shareWith":    shareWith,
		"permissions":  "31",
	}
	resp, err := RequestNextcloudOCS(cfg, "POST", "/apps/files_sharing/api/v1/shares", params)
	if err != nil {
		return fmt.Errorf("share failed: %w (resp: %s)", err, resp)
	}
	return nil
}

// ReadNextcloudFile uses WebDAV GET.
func ReadNextcloudFile(cfg *config.Config, path string) (string, error) {
	if cfg.NextcloudURL == "" || cfg.NextcloudBotUser == "" || cfg.NextcloudBotAppPassword == "" {
		return "", fmt.Errorf("nextcloud credentials not configured")
	}

    baseURL := strings.TrimRight(cfg.NextcloudURL, "/")
    user := cfg.NextcloudBotUser
    path = strings.TrimLeft(path, "/")
    davURL := fmt.Sprintf("%s/remote.php/dav/files/%s/%s", baseURL, user, path)

    req, _ := http.NewRequest("GET", davURL, nil)
    req.SetBasicAuth(user, cfg.NextcloudBotAppPassword)

    client := &http.Client{Timeout: 60 * time.Second}
    resp, err := client.Do(req)
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()

    if resp.StatusCode >= 400 {
        body, _ := io.ReadAll(resp.Body)
        return "", fmt.Errorf("WebDAV error %d: %s", resp.StatusCode, string(body))
    }

    // Limit size?
    limit := int64(100 * 1024) // 100KB
    content, err := io.ReadAll(io.LimitReader(resp.Body, limit))
    if err != nil {
        return "", err
    }
    return string(content), nil
}

// Simple XML structures for WebDAV
type MultiStatus struct {
    Responses []Response `xml:"response"`
}
type Response struct {
    Href string `xml:"href"`
}

func parseWebDavList(xmlData []byte) (string, error) {
    var ms MultiStatus
    if err := xml.Unmarshal(xmlData, &ms); err != nil {
        return string(xmlData), nil // Fallback to raw on error
    }
    var files []string
    for _, r := range ms.Responses {
        files = append(files, r.Href)
    }
    return strings.Join(files, "\n"), nil
}
