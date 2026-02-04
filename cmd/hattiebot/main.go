// HattieBot is a self-improving agent seed: OpenRouter, SQLite + sqlite-vec,
// minimal built-in tools, and instructions for the agent to create Go tools.
// The process stays running as the "brain"; the console is one interface.
// In the future, Twilio/Slack or proactive loops can attach without stopping it.
package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/hattiebot/hattiebot/internal/agent"
	"github.com/hattiebot/hattiebot/internal/agent/templates"
	"github.com/hattiebot/hattiebot/internal/bootstrap"
	"github.com/hattiebot/hattiebot/internal/channels/admin_term"
	"github.com/hattiebot/hattiebot/internal/channels/custom_webhook"
	"github.com/hattiebot/hattiebot/internal/channels/nextcloudtalk"
	"github.com/hattiebot/hattiebot/internal/config"
	"github.com/hattiebot/hattiebot/internal/core"
	"github.com/hattiebot/hattiebot/internal/gateway"
	"github.com/hattiebot/hattiebot/internal/embeddinggood"
	"github.com/hattiebot/hattiebot/internal/embeddingrouter"
	"github.com/hattiebot/hattiebot/internal/llmrouter"
	"github.com/hattiebot/hattiebot/internal/memory"
	"github.com/hattiebot/hattiebot/internal/middleware"
	"github.com/hattiebot/hattiebot/internal/openrouter"
	"github.com/hattiebot/hattiebot/internal/scheduler"
	"github.com/hattiebot/hattiebot/internal/store"
	"github.com/hattiebot/hattiebot/internal/tools"
	"github.com/hattiebot/hattiebot/internal/tools/nextcloud"
	"github.com/hattiebot/hattiebot/internal/tui"
	"github.com/hattiebot/hattiebot/internal/webhookserver"
	"github.com/hattiebot/hattiebot/internal/wiring"
)

func main() {
	cfg := config.New("")
	if err := run(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(cfg *config.Config) error {
	// First boot: no config file -> run first-boot setup, then continue (don't exit)
	cf, _ := store.LoadConfigFile(cfg.ConfigDir)
	if cf == nil {
		// Compose mode: full env-driven setup (no interactive first-boot)
		if os.Getenv("HATTIEBOT_COMPOSE_MODE") == "1" {
			apiKey := os.Getenv("OPENROUTER_API_KEY")
			model := os.Getenv("HATTIEBOT_MODEL")
			name := os.Getenv("HATTIEBOT_BOT_NAME")
			audience := os.Getenv("HATTIEBOT_AUDIENCE")
			purpose := os.Getenv("HATTIEBOT_PURPOSE")
			adminID := os.Getenv("HATTIEBOT_ADMIN_USER_ID")
			if adminID == "" {
				adminID = os.Getenv("NEXTCLOUD_ADMIN_USER")
			}
			if apiKey != "" && model != "" && name != "" && audience != "" && purpose != "" {
				seed := &store.ConfigFile{
					OpenRouterAPIKey: apiKey,
					Model:            model,
					AgentName:       name,
					AdminUserID:     adminID,
				}
				if err := store.SaveConfigFile(cfg.ConfigDir, seed); err != nil {
					return fmt.Errorf("compose seed config: %w", err)
				}
				if err := agent.WriteSoul(cfg.ConfigDir, name, audience, purpose); err != nil {
					return fmt.Errorf("compose SOUL.md: %w", err)
				}
				nextcloudURL := os.Getenv("NEXTCLOUD_URL")
				webhookSecret := os.Getenv("HATTIEBOT_WEBHOOK_SECRET")
				if nextcloudURL != "" && webhookSecret != "" {
					if err := bootstrap.WaitForNextcloud(nextcloudURL, 5*time.Minute, 15*time.Second); err != nil {
						return fmt.Errorf("nextcloud bootstrap: %w", err)
					}
					botUser := os.Getenv("NEXTCLOUD_BOT_USER")
				botPass := os.Getenv("NEXTCLOUD_BOT_APP_PASSWORD")
				
				// Auto-provision if missing and admin creds available
				adminUser := os.Getenv("NEXTCLOUD_ADMIN_USER")
				adminPass := os.Getenv("NEXTCLOUD_ADMIN_PASSWORD")
				
				if (botUser == "" || botPass == "") && adminUser != "" && adminPass != "" {
					targetBotName := name
					if targetBotName == "" {
						targetBotName = "hattiebot"
					}
					// Sanitize username
					targetBotName = strings.ToLower(strings.ReplaceAll(targetBotName, " ", ""))
					
					pUser, pPass, err := bootstrap.ProvisionBotUser(nextcloudURL, adminUser, adminPass, targetBotName)
					if err != nil {
						fmt.Fprintf(os.Stderr, "warning: failed to auto-provision bot user: %v\n", err)
					} else {
						if pPass != "" {
							botUser = pUser
							botPass = pPass
							fmt.Printf("Auto-provisioned Nextcloud user: %s\n", botUser)
						} else {
							// User exists but we don't have pass. Usage might fail if not set in config previously.
							// But maybe it was loaded from file? No, this is compose seed block.
							// If we are here, we are seeding config.
							fmt.Printf("User %s exists. Assuming password manually configured or previously set.\n", pUser)
							if botUser == "" { botUser = pUser }
						}
					}
				}

				if err := bootstrap.WriteNextcloudConfig(cfg.ConfigDir, nextcloudURL, webhookSecret, botUser, botPass); err != nil {
					return fmt.Errorf("write nextcloud config: %w", err)
				}
				
				// Update in-memory config strictly for the goroutine usage below
				cfg.NextcloudBotUser = botUser
				cfg.NextcloudBotAppPassword = botPass
				cfg.AdminUserID = adminID
				cfg.NextcloudURL = nextcloudURL

				// Auto-archive credentials in Nextcloud Passwords app (background)
				go func(c *config.Config, p, u string) {
                    if c.AdminUserID == "" { return } // No one to share with
					// Wait for Nextcloud/Passwords app to be ready (up to 5 min poll)
                    ticker := time.NewTicker(10 * time.Second)
                    defer ticker.Stop()
                    timeout := time.After(5 * time.Minute)
                    
                    for {
                        select {
                        case <-timeout:
                            fmt.Println("[Main] Timeout waiting for Nextcloud Passwords app.")
                            return
                        case <-ticker.C:
                            // Try to store secret. StoreSecret handles creation and sharing.
                            // If Passwords app is not installed, API returns error (usually 404/503/400).
                            _, err := nextcloud.StoreSecret(c, "HattieBot Credentials", p, u, c.NextcloudURL, "Auto-generated HattieBot Admin Credentials")
                            if err == nil {
                                fmt.Println("[Main] Successfully archived HattieBot credentials in Nextcloud Passwords app (folder: HattieBot Secrets). Admin: open Passwords app → Shared with you.")
                                return
                            }
							fmt.Printf("[Main] StoreSecret error: %v. Retrying...\n", err)
						}
					}
				}(cfg, botPass, botUser)

				// Init intro Talk conversation (create 1:1 room with admin, send intro)
				go func(c *config.Config, n string) {
					time.Sleep(60 * time.Second) // Allow Nextcloud/Talk to be ready (fresh install needs more time)
					if err := bootstrap.InitIntroConversation(c, n); err != nil {
						fmt.Printf("[Main] InitIntroConversation: %v\n", err)
					} else {
						fmt.Println("[Main] Intro conversation created with admin.")
					}
				}(cfg, name)

				cf, _ = store.LoadConfigFile(cfg.ConfigDir)
			}
				}
			}

		if cf == nil {
			// Optional: seed config from env for API/CI testing (no interactive first-boot)
			if os.Getenv("HATTIEBOT_SEED_CONFIG") == "1" {
				apiKey := os.Getenv("OPENROUTER_API_KEY")
				model := os.Getenv("HATTIEBOT_MODEL")
				name := os.Getenv("HATTIEBOT_BOT_NAME")
				audience := os.Getenv("HATTIEBOT_AUDIENCE")
				purpose := os.Getenv("HATTIEBOT_PURPOSE")
				if apiKey != "" && model != "" && name != "" && audience != "" && purpose != "" {
					if err := store.SaveConfigFile(cfg.ConfigDir, &store.ConfigFile{OpenRouterAPIKey: apiKey, Model: model, AgentName: name}); err != nil {
						return fmt.Errorf("seed config: %w", err)
					}
					if err := agent.WriteSoul(cfg.ConfigDir, name, audience, purpose); err != nil {
						return fmt.Errorf("seed SOUL.md: %w", err)
					}
					cf, _ = store.LoadConfigFile(cfg.ConfigDir)
				}
			}
		}
		if cf == nil {
			fmt.Fprintf(os.Stderr, "No config at %s, starting first-boot setup.\n", cfg.ConfigDir)
			os.Stderr.Sync()
			if err := tui.RunFirstBoot(cfg); err != nil {
				return err
			}
			var err error
			cf, err = store.LoadConfigFile(cfg.ConfigDir)
			if err != nil || cf == nil {
				return fmt.Errorf("config not found after first boot: %w", err)
			}
		}
	}
	// Load API key and model from config file (overrides env)
	cfg.OpenRouterAPIKey = cf.OpenRouterAPIKey
	cfg.Model = cf.Model
	cfg.AgentName = cf.AgentName
	cfg.AdminUserID = cf.AdminUserID
	if cf.EmbeddingServiceURL != "" {
		cfg.EmbeddingServiceURL = cf.EmbeddingServiceURL
	}
	if cf.EmbeddingServiceAPIKey != "" {
		cfg.EmbeddingServiceAPIKey = cf.EmbeddingServiceAPIKey
	}
	if cf.EmbeddingDimension > 0 {
		cfg.EmbeddingDimension = cf.EmbeddingDimension
	}
	cfg.NextcloudURL = cf.NextcloudURL
	cfg.HattieBridgeWebhookSecret = cf.HattieBridgeWebhookSecret
	cfg.NextcloudBotUser = cf.NextcloudBotUser
	cfg.NextcloudBotAppPassword = cf.NextcloudBotAppPassword
	if cf.NextcloudURL != "" || cf.HattieBridgeWebhookSecret != "" {
		if cfg.DefaultChannel == "" {
			cfg.DefaultChannel = "nextcloud_talk"
		}
	}
	if cf.DefaultChannel != "" {
		cfg.DefaultChannel = cf.DefaultChannel
	}

	// Fallback to env vars if config file missing them
	if cfg.OpenRouterAPIKey == "" {
		cfg.OpenRouterAPIKey = os.Getenv("OPENROUTER_API_KEY")
	}
	if cfg.Model == "" {
		cfg.Model = os.Getenv("HATTIEBOT_MODEL")
	}
	if cfg.EmbeddingServiceURL == "" {
		cfg.EmbeddingServiceURL = os.Getenv("EMBEDDING_SERVICE_URL")
	}
	if cfg.EmbeddingServiceAPIKey == "" {
		cfg.EmbeddingServiceAPIKey = os.Getenv("EMBEDDING_SERVICE_API_KEY")
	}
	if cfg.NextcloudURL == "" {
		cfg.NextcloudURL = os.Getenv("NEXTCLOUD_URL")
	}
	// Prefer env for webhook secret so Docker/compose .env is single source of truth (must match HattieBridge).
	if v := os.Getenv("HATTIEBOT_WEBHOOK_SECRET"); v != "" {
		cfg.HattieBridgeWebhookSecret = v
	} else if cfg.HattieBridgeWebhookSecret == "" {
		cfg.HattieBridgeWebhookSecret = os.Getenv("HATTIEBOT_WEBHOOK_SECRET")
	}
	if cfg.NextcloudBotUser == "" {
		cfg.NextcloudBotUser = os.Getenv("NEXTCLOUD_BOT_USER")
	}
	if cfg.NextcloudBotAppPassword == "" {
		cfg.NextcloudBotAppPassword = os.Getenv("NEXTCLOUD_BOT_APP_PASSWORD")
	}
	if cfg.DefaultChannel == "" && os.Getenv("HATTIEBOT_DEFAULT_CHANNEL") != "" {
		cfg.DefaultChannel = os.Getenv("HATTIEBOT_DEFAULT_CHANNEL")
	}
	if cfg.OpenRouterAPIKey == "" {
		return fmt.Errorf("OpenRouter API key not set: add to config or set OPENROUTER_API_KEY")
	}
	if cfg.Model == "" {
		return fmt.Errorf("model not set: add to config or set HATTIEBOT_MODEL")
	}

	// Open DB (create if missing)
	ctx := context.Background()
	db, err := store.Open(ctx, cfg.DBPath)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer db.Close()

	// Ensure templates exist in config dir (do not overwrite existing)
	if err := templates.EnsureTemplates(cfg.ConfigDir); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to ensure templates: %v\n", err)
	}

	// Load file-based context documentation
	if err := bootstrap.LoadContextDocs(ctx, db, filepath.Join(cfg.DocsDir, "context")); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to load context docs: %v\n", err)
	}

    // Init builtin tools that need DB
    tools.Init(db)

	// Load system config for modular components
	sysCfg, err := store.LoadSystemConfig(cfg.ConfigDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to load system config: %v\n", err)
	}
	// Optional: dynamic routing from llm_routing.json; fallback to single OpenRouter client
	var client core.LLMClient
	routingCfg, _ := store.LoadLLMRouting(cfg.ConfigDir)
	if routingCfg != nil && routingCfg.HasDefaultRoute() {
		bootstrap := openrouter.NewClient(cfg.OpenRouterAPIKey, cfg.Model, cfg.ConfigDir)
		client = llmrouter.NewRouterClient(routingCfg, bootstrap, cfg.ConfigDir, nil)
	} else {
		client = wiring.LoadClient(sysCfg.LLMClient, cfg.OpenRouterAPIKey, cfg.Model)
	}

	// Validate Model Configuration (prevent bricking if config.json has bad model)
	healthCtx, hCancel := context.WithTimeout(ctx, 15*time.Second)
	// We perform a minimal generation to verify model access.
	// Note: We use a lightweight prompt to minimize cost.
	_, err = client.ChatCompletion(healthCtx, []core.Message{{Role: "user", Content: "ping - respond with one word"}})
	hCancel()
	if err != nil {
		fmt.Printf("[Init] Warning: Model '%s' failed validation: %v. Falling back to env model.\n", cfg.Model, err)
		if cfg.EnvModel != "" && cfg.Model != cfg.EnvModel {
			fmt.Printf("[Init] Activating fallback model: %s\n", cfg.EnvModel)
			cfg.Model = cfg.EnvModel
			// Re-initialize client with fallback model
			if routingCfg != nil && routingCfg.HasDefaultRoute() {
				bootstrap := openrouter.NewClient(cfg.OpenRouterAPIKey, cfg.Model, cfg.ConfigDir)
				client = llmrouter.NewRouterClient(routingCfg, bootstrap, cfg.ConfigDir, nil)
			} else {
				client = wiring.LoadClient(sysCfg.LLMClient, cfg.OpenRouterAPIKey, cfg.Model)
			}
		} else {
			fmt.Println("[Init] No fallback model available or fallback matches current. Continuing with risk of failure.")
		}
	} else {
		fmt.Printf("[Init] Model '%s' verified successfully.\n", cfg.Model)
	}

	// Build embedder: embedding_routing.json default provider > single EmbeddingGood URL > LLM client Embed
	llmFallback := embeddinggood.NewLLMEmbedWrapper(client)
	var embedder core.EmbeddingClient
	embedCfg, _ := store.LoadEmbeddingRouting(cfg.ConfigDir)
	if embedCfg != nil && embedCfg.HasDefaultProvider() {
		embedder = embeddingrouter.NewRouter(embedCfg, llmFallback, nil, cfg.ConfigDir)
	} else if cfg.EmbeddingServiceURL != "" && cfg.EmbeddingServiceAPIKey != "" {
		embedder = embeddinggood.NewClient(cfg.EmbeddingServiceURL, cfg.EmbeddingServiceAPIKey, cfg.EmbeddingDimension)
	} else {
		embedder = llmFallback
	}

	// Wrap with Policy Middleware
	// Simple confirmation for now: log and approve.
	confirmFunc := func(msg string) (bool, error) {
		fmt.Printf("[POLICY] %s -> Auto-approved for verification.\n", msg)
		return true, nil
	}
	// Initial executor loading now requires client for Embedding support
	rawExecutor := wiring.LoadExecutor(sysCfg.ToolExecutor, cfg, db, client)
	truncating := middleware.NewTruncatingExecutor(rawExecutor, cfg.ToolOutputMaxRunes)
	executor := middleware.NewPolicyMiddleware(truncating, tools.BuiltinToolDefs(), confirmFunc)

	contextManager := wiring.LoadContextSelector(sysCfg.ContextSelector, db)

	// Initialize LogStore for observability
	logStore := store.NewLogStore(db.DB)
	if err := logStore.CreateTable(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to init log store: %v\n", err)
	}

	// Initialize SubmindRegistry
	submindRegistry, err := agent.LoadSubmindRegistry(cfg.ConfigDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to load submind registry: %v\n", err)
		// Try to continue with defaults loaded in LoadSubmindRegistry
	}

	loop := &agent.Loop{
		Config:          cfg,
		DB:              db,
		Executor:        executor, // Note: Executor needs loop injection below
		Client:          client,
		Context:         contextManager,
		Compactor:       memory.NewCompactor(client, 4000), // Threshold: ~4000 tokens
		SubmindRegistry: submindRegistry,
		LogStore:        logStore,
	}

	// Start scheduler background runner
	schedRunner := scheduler.NewRunner(db)
	schedRunner.ToolExecutor = executor // Wire executor for execute_tool action
	schedRunner.Start()
	defer schedRunner.Stop()

	// Gateway Setup
	gw := gateway.New(func(ctx context.Context, msg gateway.Message) (string, error) {
		// Handler: Receive message from any channel, run through agent loop
		fmt.Printf("[Gateway] Received from %s (%s): %s\n", msg.Channel, msg.SenderID, msg.Content)
		return loop.RunOneTurn(ctx, msg)
	})

	// Inject Gateway and Sub-Mind components into Executor
	loop.Gateway = gw
    // Explicitly set Spawner via interface method (safe DI)
    executor.SetSpawner(loop)

	if toolExec, ok := rawExecutor.(*tools.Executor); ok {
		toolExec.Gateway = gw
		toolExec.LogStore = logStore
		toolExec.SubmindRegistry = submindRegistry
		toolExec.Embedder = embedder
		// Spawner is now set via wrapper
	}

	// 1. Admin Terminal Channel
	gw.Register(adminterm.New())

	// 2. Nextcloud Talk Channel (if configured); webhooks from HattieBridge, send via chat API as Hattie user
	if cfg.NextcloudURL != "" && cfg.HattieBridgeWebhookSecret != "" && cfg.NextcloudBotUser != "" && cfg.NextcloudBotAppPassword != "" {
		gw.Register(nextcloudtalk.New(nextcloudtalk.Config{
			BaseURL:        cfg.NextcloudURL,
			BotUser:        cfg.NextcloudBotUser,
			BotAppPassword: cfg.NextcloudBotAppPassword,
		}))
		httpPort := 8080
		if p := os.Getenv("HATTIEBOT_HTTP_PORT"); p != "" {
			if n, err := strconv.Atoi(p); err == nil && n > 0 {
				httpPort = n
			}
		}
		if p := os.Getenv("HATTIEBOT_API_PORT"); p != "" && os.Getenv("HATTIEBOT_HTTP_PORT") == "" {
			if n, err := strconv.Atoi(p); err == nil && n > 0 {
				httpPort = n
			}
		}
		webhookSrv := &webhookserver.Server{
			Addr:               fmt.Sprintf(":%d", httpPort),
			HattieBridgeSecret: cfg.HattieBridgeWebhookSecret,
			PushIngress:        gw.PushIngress,
			ConfigDir:          cfg.ConfigDir,
		}
		defaultCh := "nextcloud_talk"
		if cfg.DefaultChannel != "" {
			defaultCh = cfg.DefaultChannel
		}
		adminID := cfg.AdminUserID
		if adminID == "" {
			adminID = "admin"
		}
		gw.Register(custom_webhook.New(gw, defaultCh, adminID))
		go func() {
			if err := webhookSrv.Run(); err != nil {
				fmt.Fprintf(os.Stderr, "webhook server: %v\n", err)
			}
		}()
	}

	// 4. Router and Escalation Monitor for proactive messaging
	router := gateway.NewRouter(gw, db)
	if cfg.DefaultChannel != "" {
		router.DefaultChannel = cfg.DefaultChannel
	}
	schedRunner.Router = router // Wire router so scheduler can deliver reminders proactively
	if toolExec, ok := rawExecutor.(*tools.Executor); ok {
		toolExec.Router = router // For notify_user tool
	}
	escalationMonitor := &scheduler.EscalationMonitor{
		DB:     db,
		Router: router,
	}
	escalationMonitor.Start(ctx, 5*time.Minute) // Check every 5 minutes

	// Start Gateway (blocks until ctx canceled)
	fmt.Println("System architecture upgraded. Gateway starting...")
	if err := gw.StartAll(ctx); err != nil {
		return err
	}

	return nil
}


func runHeadless(onSubmit func(string) (string, error)) error {
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Fprint(os.Stderr, "HattieBot (headless). Enter message:\n")
	if !scanner.Scan() {
		return scanner.Err()
	}
	msg := scanner.Text()
	if msg == "" {
		return nil
	}
	reply, err := onSubmit(msg)
	if err != nil {
		return err
	}
	fmt.Println(reply)
	return nil
}
