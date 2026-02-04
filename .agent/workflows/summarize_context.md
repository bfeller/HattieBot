---
description: How to summarize project context and update memory
---

# Context Summarization Workflow

When you have completed a significant milestone or learned important project details that should be preserved beyond the immediate chat history window (which is ~30 messages), follow this workflow.

## 1. Search History
Use `search_history` to recall specific details if needed, or simply review your recent knowledge.
- Example: `search_history(query="API key", limit=5)`

## 2. Compile Summary
Synthesize the key information into a concise markdown summary. Focus on:
- Current Project Status
- Key Decisions Made
- Important File Paths or Configs
- Blocking Issues

## 3. Update Context Document
Use `manage_context_doc` to save this summary.
- **Title**: Use a consistent title like "Project State" or specific feature names (e.g., "Auth System Design").
- **Action**: "create" (if new) or "update" (to append/refine).
- **Active**: Set `active=true` if it remains relevant for the *next* task.

## Example
```json
{
  "action": "update",
  "title": "Project State",
  "content": "# Project State (Feb 4)\n- Implemented secure secret management.\n- Webhooks now route to tools.\n- Next Step: Fix 'go' binary path in Dockerfile.",
  "active": true
}
```

// turbo
## 4. Verify
Read back the document to ensure it was saved correctly.
