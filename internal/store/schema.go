package store

const schema = `
CREATE TABLE IF NOT EXISTS users (
	id TEXT PRIMARY KEY,
	name TEXT,
	role TEXT DEFAULT 'user',
	platform TEXT,
	trust_level TEXT DEFAULT 'trusted', -- admin, trusted, guest, restricted, blocked
	metadata TEXT,
	first_seen DATETIME DEFAULT CURRENT_TIMESTAMP,
	last_seen DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS messages (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	role TEXT NOT NULL,
	content TEXT NOT NULL,
	model TEXT,
	sender_id TEXT NOT NULL,
	channel TEXT NOT NULL,
	thread_id TEXT NOT NULL,
	tool_calls TEXT,
	tool_results TEXT,
	tool_call_id TEXT,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS config (
	key TEXT PRIMARY KEY,
	value TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS tools_registry (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT NOT NULL UNIQUE,
	binary_path TEXT NOT NULL,
	description TEXT,
	input_schema TEXT,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	status TEXT DEFAULT 'active',
	last_success DATETIME,
	failure_count INTEGER DEFAULT 0,
	last_error TEXT
);

CREATE INDEX IF NOT EXISTS idx_messages_created_at ON messages(created_at);

CREATE TABLE IF NOT EXISTS jobs (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	user_id TEXT NOT NULL,
	title TEXT NOT NULL,
	description TEXT,
	status TEXT NOT NULL DEFAULT 'open', -- open, blocked, closed
	blocked_reason TEXT,
	snoozed_until DATETIME, -- NULL = not snoozed, otherwise hide until this time
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY(user_id) REFERENCES users(id)
);

CREATE TABLE IF NOT EXISTS facts (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	user_id TEXT NOT NULL,
	key TEXT NOT NULL,
	value TEXT NOT NULL,
	category TEXT,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY(user_id) REFERENCES users(id),
	UNIQUE(user_id, key)
);

CREATE TABLE IF NOT EXISTS scheduled_plans (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	user_id TEXT NOT NULL,
	description TEXT NOT NULL,
	action_type TEXT NOT NULL,
	action_payload TEXT,
	schedule_type TEXT NOT NULL,
	schedule_value TEXT,
	next_run_at DATETIME,
	last_run_at DATETIME,
	locked_until DATETIME,
	status TEXT DEFAULT 'active',
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY(user_id) REFERENCES users(id)
);

CREATE TABLE IF NOT EXISTS memory_chunks (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	content TEXT NOT NULL,
	embedding BLOB, -- JSON string or raw bytes? SQLite usually stores BLOB as raw. We will store JSON string of []float32 for portability or raw bytes? Pure Go impl -> JSON is easier to debug, BLOB is smaller. Let's use JSON string for now to avoid endianness issues. Or just BLOB.
	source TEXT,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);


CREATE TABLE IF NOT EXISTS system_logs (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
	level TEXT NOT NULL,
	component TEXT NOT NULL,
	message TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_logs_timestamp ON system_logs(timestamp);
CREATE INDEX IF NOT EXISTS idx_logs_level ON system_logs(level);
CREATE INDEX IF NOT EXISTS idx_logs_component ON system_logs(component);

CREATE TABLE IF NOT EXISTS context_documents (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	title TEXT NOT NULL UNIQUE,
	content TEXT NOT NULL,
	description TEXT,
	is_active BOOLEAN DEFAULT 0,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_context_docs_active ON context_documents(is_active);

CREATE TABLE IF NOT EXISTS submind_sessions (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	user_id TEXT NOT NULL,
	mode TEXT NOT NULL,
	task TEXT NOT NULL,
	status TEXT NOT NULL DEFAULT 'running', -- running, completed, failed, suspended
	messages TEXT NOT NULL, -- JSON array of core.Message
	turns INTEGER NOT NULL DEFAULT 0,
	result_output TEXT,
	result_error TEXT,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY(user_id) REFERENCES users(id)
);
CREATE INDEX IF NOT EXISTS idx_submind_sessions_user_status ON submind_sessions(user_id, status);

CREATE TABLE IF NOT EXISTS self_modifications (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	file_paths TEXT NOT NULL,
	change_type TEXT NOT NULL,
	description TEXT NOT NULL,
	context TEXT
);
CREATE INDEX IF NOT EXISTS idx_self_modifications_created_at ON self_modifications(created_at);

CREATE TABLE IF NOT EXISTS trusted_identities (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	type TEXT NOT NULL, -- email, phone, api_key
	value TEXT NOT NULL,
	notes TEXT,
	added_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	UNIQUE(type, value)
);
CREATE INDEX IF NOT EXISTS idx_trusted_identities_type_value ON trusted_identities(type, value);
`
