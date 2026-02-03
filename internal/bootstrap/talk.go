package bootstrap

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/hattiebot/hattiebot/internal/config"
	"github.com/hattiebot/hattiebot/internal/store"
)

// InitIntroConversation creates a 1:1 Talk room with the admin and sends an intro message.
// Called on first boot (compose mode). Skips if NextcloudIntroSent or missing config.
func InitIntroConversation(cfg *config.Config, botName string) error {
	if cfg.AdminUserID == "" || cfg.NextcloudBotUser == "" || cfg.NextcloudBotAppPassword == "" || cfg.NextcloudURL == "" {
		return nil
	}
	cf, err := store.LoadConfigFile(cfg.ConfigDir)
	if err != nil || cf == nil {
		return nil
	}
	if cf.NextcloudIntroSent {
		return nil
	}

	base := strings.TrimSuffix(cfg.NextcloudURL, "/")
	client := &http.Client{Timeout: 15 * time.Second}

	// 1. Create 1:1 room (or get existing)
	roomURL := base + "/ocs/v2.php/apps/spreed/api/v4/room"
	data := url.Values{}
	data.Set("roomType", "1")
	data.Set("invite", cfg.AdminUserID)

	req, err := http.NewRequest("POST", roomURL, strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("create room request: %w", err)
	}
	req.SetBasicAuth(cfg.NextcloudBotUser, cfg.NextcloudBotAppPassword)
	req.Header.Set("OCS-APIRequest", "true")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("create room: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("create room: %s %s", resp.Status, string(body))
	}

	// Parse OCS JSON response for token
	var ocs struct {
		OCS struct {
			Data struct {
				Token string `json:"token"`
			} `json:"data"`
		} `json:"ocs"`
	}
	if err := json.Unmarshal(body, &ocs); err != nil {
		return fmt.Errorf("parse room response: %w", err)
	}
	token := ocs.OCS.Data.Token
	if token == "" {
		// Try alternate structure (some versions wrap differently)
		var alt struct {
			OCS struct {
				Data json.RawMessage `json:"data"`
			} `json:"ocs"`
		}
		if err := json.Unmarshal(body, &alt); err == nil && len(alt.OCS.Data) > 0 {
			var dataObj struct {
				Token string `json:"token"`
			}
			if err := json.Unmarshal(alt.OCS.Data, &dataObj); err == nil && dataObj.Token != "" {
				token = dataObj.Token
			}
		}
	}
	if token == "" {
		return fmt.Errorf("no room token in response")
	}

	// 2. Send intro via chat API
	intro := fmt.Sprintf("Hi! I'm %s. I'm here to help. You can ask me anything—just start typing!", botName)
	chatURL := base + "/ocs/v2.php/apps/spreed/api/v1/chat/" + token
	chatBody := fmt.Sprintf(`{"message":%s}`, jsonEscape(intro))

	chatReq, err := http.NewRequest("POST", chatURL, strings.NewReader(chatBody))
	if err != nil {
		return fmt.Errorf("intro request: %w", err)
	}
	chatReq.SetBasicAuth(cfg.NextcloudBotUser, cfg.NextcloudBotAppPassword)
	chatReq.Header.Set("Content-Type", "application/json")
	chatReq.Header.Set("OCS-APIRequest", "true")
	chatReq.Header.Set("Accept", "application/json")

	chatResp, err := client.Do(chatReq)
	if err != nil {
		return fmt.Errorf("send intro: %w", err)
	}
	defer chatResp.Body.Close()

	if chatResp.StatusCode != http.StatusCreated {
		chatBodyRead, _ := io.ReadAll(chatResp.Body)
		return fmt.Errorf("send intro: %s %s", chatResp.Status, string(chatBodyRead))
	}

	// 3. Mark intro sent
	cf.NextcloudIntroSent = true
	return store.SaveConfigFile(cfg.ConfigDir, cf)
}

func jsonEscape(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
