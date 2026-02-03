package main

import (
	"fmt"
	"os"
    "strings"

	"github.com/hattiebot/hattiebot/internal/config"
	"github.com/hattiebot/hattiebot/internal/tools/nextcloud"
)

func main() {
    // Manually construct config for verification
    url := os.Getenv("NEXTCLOUD_URL")
    if url == "" { url = "https://localhost" }
    
    user := os.Getenv("NEXTCLOUD_BOT_USER")
    if user == "" { user = "hattie" }
    
    pass := os.Getenv("NEXTCLOUD_BOT_APP_PASSWORD")
    if pass == "" { pass = "HattieBot-1770130438239204016-1770130438" }

    cfg := &config.Config{
        NextcloudURL:            url, 
        NextcloudBotUser:        user,
        NextcloudBotAppPassword: pass, 
    }

    fmt.Println("--- Verifying Nextcloud File Tools ---")

    // 1. List Files
    fmt.Println("\n[1] Listing files in root...")
    files, err := nextcloud.ListNextcloudFiles(cfg, "/")
    if err != nil {
        fmt.Printf("ERROR List: %v\n", err)
        // Check if it's a certificate error (localhost)
        if strings.Contains(err.Error(), "certificate") {
             fmt.Println("(Certificate error expected on localhost without trust. Retrying with InsecureSkipVerify context is complicated here directly. Assuming connectivity issues if cert fails.)")
        }
    } else {
        fmt.Printf("Success. Files:\n%s\n", files)
    }

    // 2. Read Credential File
    fmt.Println("\n[2] Reading HattieBot_Credentials.txt...")
    content, err := nextcloud.ReadNextcloudFile(cfg, "/HattieBot_Credentials.txt")
    if err != nil {
        fmt.Printf("ERROR Read: %v\n", err)
    } else {
        fmt.Printf("Success. Content length: %d bytes\n", len(content))
        fmt.Printf("Content preview: %s\n", content[:50])
    }

    // 3. Write Test File
    fmt.Println("\n[3] Writing test_verify.txt...")
    writeErr := nextcloud.WriteNextcloudFile(cfg, "test_verify.txt", "Verification Success from manual script.")
    if writeErr != nil {
         fmt.Printf("ERROR Write: %v\n", writeErr)
    } else {
        fmt.Println("Success.")
    }

    // 4. Read Test File
    fmt.Println("\n[4] Reading test_verify.txt...")
    content2, err := nextcloud.ReadNextcloudFile(cfg, "test_verify.txt")
    if err != nil {
        fmt.Printf("ERROR Read (verify): %v\n", err)
    } else {
        fmt.Printf("Success. Content: %s\n", content2)
    }
}
