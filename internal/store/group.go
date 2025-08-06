package store

import (
	"fmt"
	"time"
)

var groupMigrations = []string{
	`CREATE TABLE IF NOT EXISTS groups (
		id INTEGER PRIMARY KEY,
		name TEXT NOT NULL,
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL,
		UNIQUE(name)
	);`,
	`CREATE INDEX IF NOT EXISTS idx_groups_name ON groups(name);`,
	`CREATE TABLE IF NOT EXISTS users_groups (
		id INTEGER PRIMARY KEY,
		group_id INTEGER NOT NULL,
		user_id INTEGER NOT NULL,
		FOREIGN KEY(group_id) REFERENCES groups(id) ON DELETE CASCADE,
		FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
	);`,
}

var now = time.Now().UTC().Unix()

var repeatableGroupMigrations = []string{
	fmt.Sprintf(`INSERT OR IGNORE INTO groups (id, name, created_at, updated_at) VALUES (0, 'read-only', %d, %d);`, now, now),
	fmt.Sprintf(`INSERT OR IGNORE INTO groups (id, name, created_at, updated_at) VALUES (1, 'read-write', %d, %d);`, now, now),
	fmt.Sprintf(`INSERT OR IGNORE INTO rules (id, group_id, script, sort_order, created_at, updated_at) VALUES (0, 0, 'operation == OP_OPEN && bitand(flag, O_WRITE) == 0', 0, %d, %d);`, now, now),
	fmt.Sprintf(`INSERT OR IGNORE INTO rules (id, group_id, script, sort_order, created_at, updated_at) VALUES (1, 0, 'operation == OP_STAT', 1, %d, %d);`, now, now),
	fmt.Sprintf(`INSERT OR IGNORE INTO rules (id, group_id, script, sort_order, created_at, updated_at) VALUES (2, 1, 'true', 1, %d, %d);`, now, now),
}

type Group struct {
	ID int64

	CreatedAt time.Time
	UpdatedAt time.Time

	Name string

	Rules []*Rule
}
