package store

import (
	"context"
	"database/sql"
	"time"
)

// Message represents a chat message (user, assistant, or system).
type Message struct {
	ID          int64     `json:"id"`
	Role        string    `json:"role"`
	Content     string    `json:"content"`
	Model       string    `json:"model,omitempty"`
	SenderID    string    `json:"sender_id"`
	Channel     string    `json:"channel"`
	ThreadID    string    `json:"thread_id"`
	ToolCalls   string    `json:"tool_calls,omitempty"`   // JSON
	ToolResults string    `json:"tool_results,omitempty"` // JSON
	ToolCallID  string    `json:"tool_call_id,omitempty"` // For role=tool messages
	CreatedAt   time.Time `json:"created_at"`
}

// InsertMessage inserts a message and returns its id.
func (db *DB) InsertMessage(ctx context.Context, role, content, model, senderID, channel, threadID, toolCalls, toolResults, toolCallID string) (int64, error) {
	res, err := db.ExecContext(ctx,
		`INSERT INTO messages (role, content, model, sender_id, channel, thread_id, tool_calls, tool_results, tool_call_id) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		role, content, model, senderID, channel, threadID, toolCalls, toolResults, toolCallID,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// AllMessages returns all messages ordered by created_at (full conversation history).
func (db *DB) AllMessages(ctx context.Context) ([]Message, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT id, role, content, model, sender_id, channel, thread_id, tool_calls, tool_results, created_at FROM messages ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Message
	for rows.Next() {
		var m Message
		var toolCalls, toolResults sql.NullString
		err := rows.Scan(&m.ID, &m.Role, &m.Content, &m.Model, &m.SenderID, &m.Channel, &m.ThreadID, &toolCalls, &toolResults, &m.CreatedAt)
		if err != nil {
			return nil, err
		}
		if toolCalls.Valid {
			m.ToolCalls = toolCalls.String
		}
		if toolResults.Valid {
			m.ToolResults = toolResults.String
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// RecentMessages returns the last N messages (ordered by creation).
// Filtered by threadID. Pass "" to ignore.
func (db *DB) RecentMessages(ctx context.Context, limit int, threadID string) ([]Message, error) {
	query := `SELECT id, role, content, model, sender_id, channel, thread_id, tool_calls, tool_results, tool_call_id, created_at 
		 FROM messages`
	var args []interface{}
	if threadID != "" {
		query += ` WHERE thread_id = ?`
		args = append(args, threadID)
	}
	query += ` ORDER BY created_at DESC LIMIT ?`
	args = append(args, limit)

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Message
	for rows.Next() {
		var m Message
		var toolCalls, toolResults, toolCallID sql.NullString
		err := rows.Scan(&m.ID, &m.Role, &m.Content, &m.Model, &m.SenderID, &m.Channel, &m.ThreadID, &toolCalls, &toolResults, &toolCallID, &m.CreatedAt)
		if err != nil {
			return nil, err
		}
		if toolCalls.Valid {
			m.ToolCalls = toolCalls.String
		}
		if toolResults.Valid {
			m.ToolResults = toolResults.String
		}
		if toolCallID.Valid {
			m.ToolCallID = toolCallID.String
		}
		out = append(out, m)
	}
	// Reverse to get chronological order
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, rows.Err()
}

// SearchMessages searches for messages containing the query string (case-insensitive LIKE).
func (db *DB) SearchMessages(ctx context.Context, query string, limit int) ([]Message, error) {
	q := `SELECT id, role, content, model, sender_id, channel, thread_id, tool_calls, tool_results, tool_call_id, created_at 
		 FROM messages 
		 WHERE content LIKE ? OR tool_calls LIKE ? OR tool_results LIKE ?
		 ORDER BY created_at DESC LIMIT ?`
	
	wildcard := "%" + query + "%"
	rows, err := db.QueryContext(ctx, q, wildcard, wildcard, wildcard, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Message
	for rows.Next() {
		var m Message
		var toolCalls, toolResults, toolCallID sql.NullString
		err := rows.Scan(&m.ID, &m.Role, &m.Content, &m.Model, &m.SenderID, &m.Channel, &m.ThreadID, &toolCalls, &toolResults, &toolCallID, &m.CreatedAt)
		if err != nil {
			return nil, err
		}
		if toolCalls.Valid {
			m.ToolCalls = toolCalls.String
		}
		if toolResults.Valid {
			m.ToolResults = toolResults.String
		}
		if toolCallID.Valid {
			m.ToolCallID = toolCallID.String
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// MessageStore interface for dependency injection (extendable storage).
type MessageStore interface {
	InsertMessage(ctx context.Context, role, content, model, senderID, channel, threadID, toolCalls, toolResults, toolCallID string) (int64, error)
	AllMessages(ctx context.Context) ([]Message, error)
	RecentMessages(ctx context.Context, limit int, threadID string) ([]Message, error)
	SearchMessages(ctx context.Context, query string, limit int) ([]Message, error)
}

// Ensure *DB implements MessageStore.
var _ MessageStore = (*DB)(nil)
