package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hattiebot/hattiebot/internal/store"
)

// ManageContextDocTool handles creating, updating, deleting, listing, and toggling context documents.
func ManageContextDocTool(ctx context.Context, db *store.DB, argsJSON string) (string, error) {
	var args struct {
		Action      string `json:"action"`
		Title       string `json:"title"`
		Content     string `json:"content"`
		Description string `json:"description"`
		Active      bool   `json:"active"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return ErrJSON(err), nil
	}

	switch args.Action {
	case "create":
		if args.Title == "" || args.Content == "" {
			return ErrJSON(fmt.Errorf("title and content are required for create")), nil
		}
		id, err := db.CreateContextDoc(ctx, args.Title, args.Content, args.Description)
		if err != nil {
			return ErrJSON(err), nil
		}
		return fmt.Sprintf(`{"status": "created", "id": %d, "title": "%s"}`, id, args.Title), nil

	case "update":
		if args.Title == "" {
			return ErrJSON(fmt.Errorf("title is required for update")), nil
		}
		// Fetch existing to ensure it exists? UpdateContextDoc works directly.
		if err := db.UpdateContextDoc(ctx, args.Title, args.Content, args.Description); err != nil {
			return ErrJSON(err), nil
		}
		return fmt.Sprintf(`{"status": "updated", "title": "%s"}`, args.Title), nil

	case "delete":
		if args.Title == "" {
			return ErrJSON(fmt.Errorf("title is required for delete")), nil
		}
		if err := db.DeleteContextDoc(ctx, args.Title); err != nil {
			return ErrJSON(err), nil
		}
		return fmt.Sprintf(`{"status": "deleted", "title": "%s"}`, args.Title), nil

	case "list":
		docs, err := db.ListContextDocs(ctx)
		if err != nil {
			return ErrJSON(err), nil
		}
		// Return simplified list
		type DocSummary struct {
			Title       string `json:"title"`
			Description string `json:"description"`
			IsActive    bool   `json:"is_active"`
		}
		var summaries []DocSummary
		for _, d := range docs {
			summaries = append(summaries, DocSummary{
				Title:       d.Title,
				Description: d.Description,
				IsActive:    d.IsActive,
			})
		}
		b, _ := json.Marshal(summaries)
		return string(b), nil

	case "read":
		if args.Title == "" {
			return ErrJSON(fmt.Errorf("title is required for read")), nil
		}
		doc, err := db.GetContextDoc(ctx, args.Title)
		if err != nil {
			return ErrJSON(err), nil
		}
		if doc == nil {
			return `{"error": "not found"}`, nil
		}
		b, _ := json.Marshal(doc)
		return string(b), nil

	case "toggle":
		if args.Title == "" {
			return ErrJSON(fmt.Errorf("title is required for toggle")), nil
		}
		if err := db.SetContextDocActive(ctx, args.Title, args.Active); err != nil {
			return ErrJSON(err), nil
		}
		status := "inactive"
		if args.Active {
			status = "active"
		}
		return fmt.Sprintf(`{"status": "%s", "title": "%s"}`, status, args.Title), nil

	default:
		return ErrJSON(fmt.Errorf("unknown action: %s", args.Action)), nil
	}
}

