package store

import (
	"time"
)

var ruleMigrations = []string{
	`CREATE TABLE IF NOT EXISTS rules (
		id INTEGER PRIMARY KEY,
		sort_order INTEGER NOT NULL,
		group_id INTEGER NOT NULL,
		script TEXT NOT NULL,
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL,
		FOREIGN KEY(group_id) REFERENCES groups(id) ON DELETE CASCADE
	);`,
}

type Rule struct {
	ID int64

	CreatedAt time.Time
	UpdatedAt time.Time

	Script    string
	SortOrder int

	Group *Group
}
