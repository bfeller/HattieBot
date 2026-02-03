# HattieBot Chat Testing Guide

This document provides message-based tests you can perform by chatting with HattieBot (e.g., via Nextcloud Talk). Each test is designed to exercise a specific capability. Copy the message into chat and verify the expected behavior.

---

## Prerequisites

- HattieBot is running with Nextcloud integration
- You are chatting as the admin user in Nextcloud Talk
- Hattie has already sent its intro message (first boot flow)

---

## 1. Communication & Basic Response

| # | Message to Send | Expected Behavior |
|---|-----------------|-------------------|
| 1.1 | `Hello!` | Hattie responds with a greeting |
| 1.2 | `What's your name?` | Hattie introduces itself using its SOUL.md identity |
| 1.3 | `What can you do?` | Hattie describes its capabilities (tools, memory, etc.) |

---

## 2. Filesystem & Workspace

| # | Message to Send | Expected Behavior |
|---|-----------------|-------------------|
| 2.1 | `List the files in the workspace root` | Hattie uses `list_dir` and shows directory contents |
| 2.2 | `Read the file README.md` | Hattie uses `read_file` and displays the contents |
| 2.3 | `Create a file called test.txt with the content "Hello from Hattie"` | Hattie uses `write_file` and confirms creation |
| 2.4 | `Read test.txt to verify it was created` | Hattie reads and shows the file content |

---

## 3. Memory & User Preferences

| # | Message to Send | Expected Behavior |
|---|-----------------|-------------------|
| 3.1 | `Remember that my favorite color is blue` | Hattie uses `manage_user_preference` (set) |
| 3.2 | `What's my favorite color?` | Hattie recalls the fact via `manage_user_preference` (get) |
| 3.3 | `Memorize this: The project uses Go 1.22+ and Docker` | Hattie uses `memorize` to store in vector memory |
| 3.4 | `What do you remember about the project setup?` | Hattie uses `recall_memories` and retrieves relevant chunks |

---

## 4. Task Management (Jobs / Epic Memory)

| # | Message to Send | Expected Behavior |
|---|-----------------|-------------------|
| 4.1 | `Create a job called "Refactor API" with description "Improve the REST endpoints"` | Hattie uses `manage_job` create |
| 4.2 | `List my open jobs` | Hattie lists jobs via `manage_job` list |
| 4.3 | `Update job 1 to blocked because we're waiting on a dependency` | Hattie uses `manage_job` update |
| 4.4 | `Snooze job 1 for 2 hours` | Hattie uses `manage_job` snooze (if supported in definition) |

---

## 5. Scheduling & Reminders

| # | Message to Send | Expected Behavior |
|---|-----------------|-------------------|
| 5.1 | `Remind me in 1 hour to take a break` | Hattie uses `manage_schedule` create (once) |
| 5.2 | `List my scheduled reminders` | Hattie uses `manage_schedule` list |
| 5.3 | `Remind me every day at 9am to check email` | Hattie uses `manage_schedule` create (daily) |
| 5.4 | `Schedule a daily self-reflection at 9am` | Hattie uses `manage_schedule` create with action_type=agent_prompt |
| 5.5 | `Schedule a daily autonomous task to check logs and only notify me if there are errors` | Hattie uses `manage_schedule` create with action_type=agent_prompt, autonomous=true |

---

## 6. Terminal & Commands

| # | Message to Send | Expected Behavior |
|---|-----------------|-------------------|
| 6.1 | `Run the command: echo hello` | Hattie uses `run_terminal_cmd` and shows output |
| 6.2 | `What's the current date? Run date` | Hattie runs `date` and reports the result |
| 6.3 | `List the contents of the current directory with ls -la` | Hattie runs `ls -la` and shows output |

---

## 7. Registered Tools (if available)

| # | Message to Send | Expected Behavior |
|---|-----------------|-------------------|
| 7.1 | `Use the weather tool to get the weather for London` | Hattie uses `execute_registered_tool` with `weather` (if registered) |
| 7.2 | `Fetch the content of https://example.com` | Hattie uses `fetch_url` or `webread` (if registered) |
| 7.3 | `List the registered tools` | Hattie may use `read_logs` or internal knowledge; or you can ask "what tools do you have" |

---

## 8. System Status & Logs

| # | Message to Send | Expected Behavior |
|---|-----------------|-------------------|
| 8.1 | `What's the system status?` | Hattie uses `system_status` and summarizes health |
| 8.2 | `Show me recent error logs` | Hattie uses `read_logs` with level=error |
| 8.3 | `Run a self-reflection` | Hattie uses `self_reflect` and reports analysis |

---

## 9. Sub-Minds

| # | Message to Send | Expected Behavior |
|---|-----------------|-------------------|
| 9.1 | `List available sub-mind modes` | Hattie uses `manage_submind` list |
| 9.2 | `Spawn a reflection sub-mind to analyze our conversation so far` | Hattie uses `spawn_submind` with mode=reflection |
| 9.3 | `Spawn a planning sub-mind to outline steps for adding a new feature` | Hattie uses `spawn_submind` with mode=planning |

---

## 10. Nextcloud Integration

| # | Message to Send | Expected Behavior |
|---|-----------------|-------------------|
| 10.1 | `List my Nextcloud files in the root` | Hattie uses `list_nextcloud_files` with path=/ |
| 10.2 | `Read the file Documents/notes.txt from Nextcloud` | Hattie uses `read_nextcloud_file` |
| 10.3 | `Store a secret in Nextcloud Passwords: title "Test API Key", password "abc123"` | Hattie uses `store_secret` (requires Passwords app) |
| 10.4 | `Get the secret for "HattieBot" from Nextcloud Passwords` | Hattie uses `get_secret` (bot credentials stored at first boot) |

---

## 10.5 Configurable Webhooks

| # | Message to Send | Expected Behavior |
|---|-----------------|-------------------|
| 10.5.1 | `List my webhook routes` | Hattie uses `list_webhook_routes` |
| 10.5.2 | `Add a webhook route for GitHub at /webhook/github, id github, secret header X-Hub-Signature-256, secret env GITHUB_WEBHOOK_SECRET, auth type hmac_sha256` | Hattie uses `add_webhook_route` (user must set GITHUB_WEBHOOK_SECRET in env) |
| 10.5.3 | `Remove the webhook route for github` | Hattie uses `remove_webhook_route` |

---

## 11. Architecture & Documentation

| # | Message to Send | Expected Behavior |
|---|-----------------|-------------------|
| 11.1 | `Read the architecture docs` | Hattie uses `read_architecture` and summarizes |
| 11.2 | `How do I create a new tool?` | Hattie references docs/creating-tools.md or architecture |

---

## 12. Tool Creation (Advanced)

| # | Message to Send | Expected Behavior |
|---|-----------------|-------------------|
| 12.1 | `Create a tool that echoes its input. Use autohand to write the Go code, build it, and register it` | Hattie uses `autohand_cli`, `run_terminal_cmd`, `register_tool` |
| 12.2 | `Run the echo tool with input "hello"` | Hattie uses `execute_registered_tool` with the newly created tool |

---

## 13. Admin-Only Tools (Admin User)

| # | Message to Send | Expected Behavior |
|---|-----------------|-------------------|
| 13.1 | `List all users` | Hattie uses `list_users` |
| 13.2 | `Add a new LLM provider for Ollama` | Hattie uses `manage_llm_provider` (admin) |
| 13.3 | `List embedding providers` | Hattie uses `manage_embedding_provider` list_providers |

---

## 14. First Boot & Bootstrap (Fresh Install)

Perform these after a full reset (`start.sh -a -l`):

| # | Check | Expected Behavior |
|---|-------|-------------------|
| 14.1 | Open Nextcloud Talk as admin | Hattie has already started a 1:1 conversation with an intro message |
| 14.2 | Open Nextcloud Passwords app | "HattieBot Credentials" (or similar) is visible in shared passwords |
| 14.3 | Send "Hello" in that conversation | Hattie receives the message via HattieBridge webhook and replies |

---

## 15. Edge Cases & Error Handling

| # | Message to Send | Expected Behavior |
|---|-----------------|-------------------|
| 15.1 | `Read the file /etc/passwd` | Hattie may refuse or report path not in workspace |
| 15.2 | `Run: rm -rf /` | Hattie refuses (destructive command) |
| 15.3 | `Read a file that doesn't exist: nonexistent.txt` | Hattie reports file not found |
| 15.4 | `Get secret for "nonexistent"` | Hattie reports secret not found |

---

## Quick Smoke Test (Minimal)

If you only have time for a few tests, run these in order:

1. **1.1** – `Hello!` (basic response)
2. **2.1** – `List the files in the workspace root` (filesystem)
3. **3.1** – `Remember that my favorite color is blue` (user preference)
4. **3.2** – `What's my favorite color?` (recall)
5. **8.1** – `What's the system status?` (system_status)
6. **10.1** – `List my Nextcloud files in the root` (Nextcloud, if configured)

---

## Troubleshooting

- **No response**: Check HattieBridge webhook is receiving messages; verify `HATTIE_BRIDGE_WEBHOOK_SECRET` matches in Nextcloud app config and HattieBot env.
- **"Config not available"**: Nextcloud tools require `NEXTCLOUD_URL`, `NEXTCLOUD_BOT_USER`, `NEXTCLOUD_BOT_APP_PASSWORD`.
- **Passwords 404**: Ensure Passwords app is installed and `XDG_CACHE_HOME` is set for Nextcloud (Fontconfig fix).
- **Tool not found**: Registered tools must exist in `$CONFIG_DIR/tools` and be built to `$CONFIG_DIR/bin`.
