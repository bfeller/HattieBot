package bootstrap

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/hattiebot/hattiebot/internal/store"
)

// ProvisionBotUser ensures a user exists in Nextcloud for the bot, added to the "admin" group.
// It uses the provided admin credentials to query/create via the OCS Provisioning API.
// Returns the bot username and app password (if newly created, or existing if we can manage that - actually for OCS we set a password).
// For the bot to have API access, we need to create it.
// If it exists, we generate a new App Password for it if possible, or we just return the user/pass if we set it.
// Actually, OCS create user takes a password. We'll generate a random one if creating.
// To get an App Password for *itself* effectively, using Basic Auth with the main password works for OCS/WebDAV.
// So we will return the main password generated.
func ProvisionBotUser(baseURL, adminUser, adminPass, botName string) (string, string, error) {
	// 1. Check if user exists
	client := &http.Client{Timeout: 10 * time.Second}
	u := strings.TrimRight(baseURL, "/")
	checkURL := fmt.Sprintf("%s/ocs/v1.php/cloud/users/%s", u, botName)

	req, _ := http.NewRequest("GET", checkURL, nil)
	req.SetBasicAuth(adminUser, adminPass)
	req.Header.Set("OCS-APIRequest", "true")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("check users: %w", err)
	}
	defer resp.Body.Close()

    // OCS Response Struct
    type OCSMeta struct {
        Status     string `json:"status"`
        StatusCode int    `json:"statuscode"`
        Message    string `json:"message"`
    }
    type OCSResponse struct {
        OCS struct {
            Meta OCSMeta `json:"meta"`
            Data interface{} `json:"data"`
        } `json:"ocs"`
    }

    bodyBytes, _ := io.ReadAll(resp.Body)
    var ocsResp OCSResponse
    // Try to decode JSON. If not JSON (e.g. HTML 404), fallback to status code check.
    jsonErr := json.Unmarshal(bodyBytes, &ocsResp)

    // Helper to check OCS failure
    isOCSSuccess := jsonErr == nil && ocsResp.OCS.Meta.StatusCode == 100
    isOCSNotFound := jsonErr == nil && (ocsResp.OCS.Meta.StatusCode == 996 || ocsResp.OCS.Meta.StatusCode == 998 || ocsResp.OCS.Meta.StatusCode == 404)

	var generatedPass string
	
    // 1. User Exists (OCS 100)
	if resp.StatusCode == 200 && isOCSSuccess {
		// User exists. Reset password to ensure we have access.
		log.Printf("[Bootstrap] User %s exists. Resetting password...", botName)
		generatedPass = fmt.Sprintf("HattieBot-%d-%d", time.Now().UnixNano(), time.Now().Unix())
		
		maxRetries := 3
		for i := 0; i < maxRetries; i++ {
			editURL := fmt.Sprintf("%s/ocs/v1.php/cloud/users/%s", u, botName)
			data := url.Values{}
			data.Set("password", generatedPass)
			
			req, _ := http.NewRequest("PUT", editURL, strings.NewReader(data.Encode()))
			req.SetBasicAuth(adminUser, adminPass)
			req.Header.Set("OCS-APIRequest", "true")
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req.Header.Set("Accept", "application/json")
			
			editResp, err := client.Do(req)
			if err != nil {
				if i == maxRetries-1 {
					return "", "", fmt.Errorf("edit user password: %w", err)
				}
				time.Sleep(1 * time.Second)
				continue
			}
			defer editResp.Body.Close()
            
            eBody, _ := io.ReadAll(editResp.Body)
            var eOcs OCSResponse
            json.Unmarshal(eBody, &eOcs)
			
			if editResp.StatusCode == 200 && eOcs.OCS.Meta.StatusCode == 100 {
				log.Printf("[Bootstrap] Password for %s reset successfully.", botName)
				return botName, generatedPass, nil
			}
			
			if i == maxRetries-1 {
				return "", "", fmt.Errorf("edit user password failed (%d/%d): %s", editResp.StatusCode, eOcs.OCS.Meta.StatusCode, eOcs.OCS.Meta.Message)
			}
			time.Sleep(1 * time.Second)
		}
		return botName, "", fmt.Errorf("failed to reset password after retries")

	} else if resp.StatusCode == 404 || isOCSNotFound || (jsonErr == nil && ocsResp.OCS.Meta.StatusCode == 997) { 
        // 997 is Unauthorised, but usually means we can't see the user? 
        // No, 997 means ADMIN creds are bad. We should NOT try to create if admin creds are bad.
        if jsonErr == nil && ocsResp.OCS.Meta.StatusCode == 997 {
             return "", "", fmt.Errorf("admin authentication failed (OCS 997): check NEXTCLOUD_ADMIN_USER/PASSWORD")
        }

		// Create user
		log.Printf("[Bootstrap] Creating Nextcloud user %s...", botName)
		createURL := fmt.Sprintf("%s/ocs/v1.php/cloud/users", u)
		
		// Generate random password
		generatedPass = fmt.Sprintf("HattieBot-%d-%d", time.Now().UnixNano(), time.Now().Unix())
		
		data := url.Values{}
		data.Set("userid", botName)
		data.Set("password", generatedPass)
		data.Set("groups[]", "admin") // Add to admin group!

		req, _ := http.NewRequest("POST", createURL, strings.NewReader(data.Encode()))
		req.SetBasicAuth(adminUser, adminPass)
		req.Header.Set("OCS-APIRequest", "true")
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Accept", "application/json")

		createResp, err := client.Do(req)
		if err != nil {
			return "", "", fmt.Errorf("create user: %w", err)
		}
		defer createResp.Body.Close()
        
        cBody, _ := io.ReadAll(createResp.Body)
        var cOcs OCSResponse
        json.Unmarshal(cBody, &cOcs)

		if createResp.StatusCode == 200 && cOcs.OCS.Meta.StatusCode == 100 {
            log.Printf("[Bootstrap] User %s created successfully.", botName)
		    return botName, generatedPass, nil
		}
        
		return "", "", fmt.Errorf("create user failed (%d/%d): %s", createResp.StatusCode, cOcs.OCS.Meta.StatusCode, cOcs.OCS.Meta.Message)
	} else {
		// Unknown error
		return "", "", fmt.Errorf("check user %s failed (%d/%d): %s", botName, resp.StatusCode, ocsResp.OCS.Meta.StatusCode, ocsResp.OCS.Meta.Message)
	}
}

func WaitForNextcloud(baseURL string, timeout, interval time.Duration) error {
	url := baseURL
	if len(url) > 0 && url[len(url)-1] == '/' {
		url = url[:len(url)-1]
	}
	url = url + "/status.php"
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 10 * time.Second}
	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err != nil {
			log.Printf("[Bootstrap] Nextcloud not ready: %v", err)
			time.Sleep(interval)
			continue
		}
		_ = resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			return nil
		}
		log.Printf("[Bootstrap] Nextcloud status %d", resp.StatusCode)
		time.Sleep(interval)
	}
	return fmt.Errorf("nextcloud not ready within %v", timeout)
}

// WriteNextcloudConfig merges Nextcloud fields into the config file at dir.
func WriteNextcloudConfig(dir, nextcloudURL, webhookSecret, botUser, botAppPassword string) error {
	cf, err := store.LoadConfigFile(dir)
	if err != nil {
		return err
	}
	if cf == nil {
		cf = &store.ConfigFile{}
	}
	cf.NextcloudURL = nextcloudURL
	cf.HattieBridgeWebhookSecret = webhookSecret
	if botUser != "" {
		cf.NextcloudBotUser = botUser
	}
	if botAppPassword != "" {
		cf.NextcloudBotAppPassword = botAppPassword
	}
	return store.SaveConfigFile(dir, cf)
}
