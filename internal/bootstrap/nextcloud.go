package bootstrap

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/hattiebot/hattiebot/internal/store"
)

// WaitForNextcloud polls url/status.php until it returns 200 and indicates "installed", or timeout.
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
func WriteNextcloudConfig(dir, nextcloudURL, talkSecret, botUser, botAppPassword string) error {
	cf, err := store.LoadConfigFile(dir)
	if err != nil {
		return err
	}
	if cf == nil {
		cf = &store.ConfigFile{}
	}
	cf.NextcloudURL = nextcloudURL
	cf.NextcloudTalkBotSecret = talkSecret
	if botUser != "" {
		cf.NextcloudBotUser = botUser
	}
	if botAppPassword != "" {
		cf.NextcloudBotAppPassword = botAppPassword
	}
	return store.SaveConfigFile(dir, cf)
}
