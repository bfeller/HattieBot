package nextcloud

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/hattiebot/hattiebot/internal/config"
)

// Passwords App API (v51+)
// Documentation: https://git.mdns.eu/nextcloud/passwords/-/wikis/Api/Index

// GetNextcloudSecret searches for a password by title.
func GetNextcloudSecret(cfg *config.Config, query string) (string, error) {
	// API route: /api/1.0/password/list (requires session). Filter client-side.
	baseURL := strings.TrimRight(cfg.NextcloudURL, "/")
	apiURL := fmt.Sprintf("%s/index.php/apps/passwords/api/1.0/password/list", baseURL)

	req, _ := http.NewRequest("GET", apiURL, nil)
	req.SetBasicAuth(cfg.NextcloudBotUser, cfg.NextcloudBotAppPassword)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("OCS-APIRequest", "true")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
    
    if resp.StatusCode != 200 {
        body, _ := io.ReadAll(resp.Body)
        return "", fmt.Errorf("passwords API error %d: %s", resp.StatusCode, string(body))
    }

    // Parse JSON
    var list []map[string]interface{}
    if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
        return "", fmt.Errorf("parse error: %v", err)
    }

    // Filter
    for _, item := range list {
        title, _ := item["label"].(string) // "label" alias "title" in some versions
        if title == "" {
             title, _ = item["title"].(string)
        }
        if strings.Contains(strings.ToLower(title), strings.ToLower(query)) {
            // Found. Get details (password might be hidden or in detailed view).
            // Usually list returns details including 'password' field if authorized.
            pass, _ := item["password"].(string)
            login, _ := item["username"].(string)
            return fmt.Sprintf("Title: %s\nUser: %s\nPass: %s", title, login, pass), nil
        }
    }

	return "", fmt.Errorf("secret not found for query: %s", query)
}

// GetSecretValue searches for a password by exact label/title match and returns the password string.
func GetSecretValue(cfg *config.Config, label string) (string, error) {
	// API route: /api/1.0/password/list
	baseURL := strings.TrimRight(cfg.NextcloudURL, "/")
	apiURL := fmt.Sprintf("%s/index.php/apps/passwords/api/1.0/password/list", baseURL)

	req, _ := http.NewRequest("GET", apiURL, nil)
	req.SetBasicAuth(cfg.NextcloudBotUser, cfg.NextcloudBotAppPassword)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("OCS-APIRequest", "true")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	
    if resp.StatusCode != 200 {
        body, _ := io.ReadAll(resp.Body)
        return "", fmt.Errorf("passwords API error %d: %s", resp.StatusCode, string(body))
    }

    var list []map[string]interface{}
    if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
        return "", fmt.Errorf("parse error: %v", err)
    }

    // Exact match preference, then case-insensitive
    for _, item := range list {
        t, _ := item["label"].(string)
        if t == "" { t, _ = item["title"].(string) }
        
        if t == label {
            if pass, ok := item["password"].(string); ok {
                return pass, nil
            }
        }
    }
    // Fallback case-insensitive
    for _, item := range list {
        t, _ := item["label"].(string)
        if t == "" { t, _ = item["title"].(string) }
        if strings.EqualFold(t, label) {
             if pass, ok := item["password"].(string); ok {
                return pass, nil
            }
        }
    }

	return "", fmt.Errorf("secret not found: %s", label)
}

// StoreSecret creates a new password and shares it with admin.
// detailed session handshake is used.
// If the Passwords App API fails (e.g. 404/500), it falls back to creating a secure text file and sharing it.
func StoreSecret(cfg *config.Config, title, password, login, targetURL, notes string) (string, error) {
    if cfg.NextcloudURL == "" || cfg.NextcloudBotUser == "" || cfg.NextcloudBotAppPassword == "" {
		return "", fmt.Errorf("nextcloud credentials not configured")
	}
    
    // 1. Try API Storage (Session Handshake)
    apiID, err := storeSecretViaAPI(cfg, title, password, login, targetURL, notes)
    if err == nil {
        return apiID, nil
    }
    
    // API Failed - No Fallback per user request
    return "", fmt.Errorf("api failed: %w", err)
}

const baseFolderUUID = "00000000-0000-0000-0000-000000000000"
const secretsFolderLabel = "HattieBot Secrets"

// createOrGetSecretsFolder returns the UUID of the "HattieBot Secrets" folder in the Passwords app.
func createOrGetSecretsFolder(client *http.Client, effectiveBase, sessionToken string, sessionCookie *http.Cookie, cfg *config.Config) (string, error) {
	// List folders and find by label
	listURL := effectiveBase + "/folder/list"
	req, _ := http.NewRequest("POST", listURL, strings.NewReader("{}"))
	req.SetBasicAuth(cfg.NextcloudBotUser, cfg.NextcloudBotAppPassword)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("OCS-APIRequest", "true")
	req.Header.Set("X-API-SESSION", sessionToken)
	if sessionCookie != nil {
		req.AddCookie(sessionCookie)
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return baseFolderUUID, nil // Fallback to root if list fails
	}
	var folders []map[string]interface{}
	if err := json.Unmarshal(body, &folders); err != nil {
		return baseFolderUUID, nil
	}
	for _, f := range folders {
		if lab, _ := f["label"].(string); lab == secretsFolderLabel {
			if id, _ := f["id"].(string); id != "" {
				return id, nil
			}
		}
	}
	// Create folder
	createPayload := map[string]interface{}{
		"label":   secretsFolderLabel,
		"parent":  baseFolderUUID,
		"cseType": "none",
	}
	createData, _ := json.Marshal(createPayload)
	createReq, _ := http.NewRequest("POST", effectiveBase+"/folder/create", strings.NewReader(string(createData)))
	createReq.SetBasicAuth(cfg.NextcloudBotUser, cfg.NextcloudBotAppPassword)
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("Accept", "application/json")
	createReq.Header.Set("OCS-APIRequest", "true")
	createReq.Header.Set("X-API-SESSION", sessionToken)
	if sessionCookie != nil {
		createReq.AddCookie(sessionCookie)
	}
	createResp, err := client.Do(createReq)
	if err != nil {
		return baseFolderUUID, nil
	}
	defer createResp.Body.Close()
	createBody, _ := io.ReadAll(createResp.Body)
	if createResp.StatusCode >= 300 {
		return baseFolderUUID, nil
	}
	var createResult struct {
		ID string `json:"id"`
	}
	if json.Unmarshal(createBody, &createResult) == nil && createResult.ID != "" {
		return createResult.ID, nil
	}
	return baseFolderUUID, nil
}

// storeSecretViaAPI implements the Session+Create+Share flow for the Passwords App
func storeSecretViaAPI(cfg *config.Config, title, password, login, targetURL, notes string) (string, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	baseURL := strings.TrimRight(cfg.NextcloudURL, "/")
	sessionPaths := []string{
		fmt.Sprintf("%s/index.php/apps/passwords/api/1.0/session/open", baseURL),
		fmt.Sprintf("%s/apps/passwords/api/1.0/session/open", baseURL),
	}

	var sessionToken string
	var sessionCookie *http.Cookie
	var effectiveBase string

	for _, sUrl := range sessionPaths {
		sReq, _ := http.NewRequest("POST", sUrl, strings.NewReader("{}"))
		sReq.SetBasicAuth(cfg.NextcloudBotUser, cfg.NextcloudBotAppPassword)
		sReq.Header.Set("Content-Type", "application/json")
		sReq.Header.Set("Accept", "application/json")
		sReq.Header.Set("OCS-APIRequest", "true")

		sResp, err := client.Do(sReq)
		if err != nil {
			continue
		}
		defer sResp.Body.Close()

		if sResp.StatusCode == 200 {
			sessionToken = sResp.Header.Get("X-API-SESSION")
			for _, c := range sResp.Cookies() {
				if c.Name == "nc_passwords" {
					sessionCookie = c
					break
				}
			}
			effectiveBase = strings.Replace(sUrl, "/session/open", "", 1)
			break
		}
	}

	if sessionToken == "" {
		return "", fmt.Errorf("start session failed")
	}

	// Create or get the shared secrets folder
	folderID, _ := createOrGetSecretsFolder(client, effectiveBase, sessionToken, sessionCookie, cfg)

	// Create Secret (API route: /api/1.0/password/create)
	payload := map[string]interface{}{
		"label":    title,
		"password": password,
		"username": login,
		"url":      targetURL,
		"notes":    notes,
		"cseType":  "none",
		"folder":   folderID,
	}

    data, _ := json.Marshal(payload)
    createURL := effectiveBase + "/password/create"
    
    req, _ := http.NewRequest("POST", createURL, strings.NewReader(string(data)))
    req.SetBasicAuth(cfg.NextcloudBotUser, cfg.NextcloudBotAppPassword)
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Accept", "application/json")
    req.Header.Set("OCS-APIRequest", "true")
    req.Header.Set("X-API-SESSION", sessionToken)
    if sessionCookie != nil {
        req.AddCookie(sessionCookie)
    }

    resp, err := client.Do(req)
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()
    
    respBody, _ := io.ReadAll(resp.Body)
    if resp.StatusCode >= 300 {
         return "", fmt.Errorf("create secret failed %d: %s", resp.StatusCode, string(respBody))
    }

    var result struct {
        ID string `json:"id"`
    }
    if err := json.Unmarshal(respBody, &result); err != nil {
         return "Stored (parse error)", nil
    }
    
    // Share with Admin (API route: /api/1.0/share/create)
    if cfg.AdminUserID != "" && result.ID != "" {
        shareURL := effectiveBase + "/share/create"
        sharePayload := map[string]interface{}{
            "password":  result.ID,
            "receiver":  cfg.AdminUserID,
            "editable":  true,
            "shareable": true,
        }
        sData, _ := json.Marshal(sharePayload)
        sReq, _ := http.NewRequest("POST", shareURL, strings.NewReader(string(sData)))
        sReq.SetBasicAuth(cfg.NextcloudBotUser, cfg.NextcloudBotAppPassword)
        sReq.Header.Set("Content-Type", "application/json")
        sReq.Header.Set("X-API-SESSION", sessionToken)
        sReq.Header.Set("OCS-APIRequest", "true")
        if sessionCookie != nil {
            sReq.AddCookie(sessionCookie)
        }
        shareResp, err := client.Do(sReq)
        if shareResp != nil {
            shareResp.Body.Close()
        }
        if err == nil && shareResp != nil && shareResp.StatusCode >= 200 && shareResp.StatusCode < 300 {
            // Trigger Passwords app share sync so admin sees the share immediately
            triggerPasswordsShareSync(cfg)
        }
    }

    return result.ID, nil
}

// triggerPasswordsShareSync triggers the Passwords app's share sync so the admin sees shared passwords immediately.
func triggerPasswordsShareSync(cfg *config.Config) {
    if cfg.NextcloudURL == "" || cfg.NextcloudBotUser == "" || cfg.NextcloudBotAppPassword == "" {
        return
    }
    syncURL := strings.TrimRight(cfg.NextcloudURL, "/") + "/index.php/apps/passwords/cron/sharing"
    req, _ := http.NewRequest("GET", syncURL, nil)
    req.SetBasicAuth(cfg.NextcloudBotUser, cfg.NextcloudBotAppPassword)
    c := &http.Client{Timeout: 15 * time.Second}
    resp, err := c.Do(req)
    if err != nil {
        return
    }
    resp.Body.Close()
}
