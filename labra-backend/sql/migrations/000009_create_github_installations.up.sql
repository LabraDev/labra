CREATE TABLE IF NOT EXISTS github_installations (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id INTEGER NOT NULL UNIQUE,
  installation_id INTEGER NOT NULL,
  account_login TEXT,
  target_type TEXT,
  created_at INTEGER NOT NULL DEFAULT (unixepoch()),
  updated_at INTEGER NOT NULL DEFAULT (unixepoch())
);

CREATE INDEX IF NOT EXISTS idx_github_installations_installation
  ON github_installations(installation_id);
