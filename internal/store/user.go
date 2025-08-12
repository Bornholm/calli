package store

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/bornholm/calli/internal/authz"
	"github.com/bornholm/calli/internal/authz/expr"
	"github.com/pkg/errors"
	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

var userMigrations = []string{
	`CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY,
		
		subject TEXT,
		provider TEXT,

		nickname TEXT,
		email TEXT,

		is_admin BOOLEAN,

		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL,
		connected_at INTEGER,
		
		basic_username TEXT,
		basic_password BLOB,

		UNIQUE (subject, provider),
		UNIQUE (basic_username)
	);`,
}

type User struct {
	ID int64

	Provider string
	Subject  string

	IsAdmin bool

	CreatedAt   time.Time
	UpdatedAt   time.Time
	ConnectedAt time.Time

	Nickname string
	Email    string

	BasicUsername string
	BasicPassword []byte

	groups []*Group
}

// Groups implements authz.User.
func (u *User) FileSystemGroups() []*authz.Group {
	return slices.Collect(func(yield func(*authz.Group) bool) {
		for _, g := range u.groups {
			rules := slices.Collect(func(yield func(authz.Rule) bool) {
				for _, r := range g.Rules {
					if !yield(expr.NewRule(r.Script)) {
						return
					}
				}
			})
			if !yield(authz.NewGroup(g.Name, rules...)) {
				return
			}
		}
	})
}

// FileSystemRules implements authz.User.
func (u *User) FileSystemRules() []authz.Rule {
	rules := make([]authz.Rule, 0)

	if u.IsAdmin {
		// An admin can access everything on any filesystem
		rules = append(rules, expr.NewRule("true"))
	}

	groupRules := slices.Collect(func(yield func(authz.Rule) bool) {
		for _, g := range u.groups {
			for _, r := range g.Rules {
				if !yield(expr.NewRule(r.Script)) {
					return
				}
			}
		}
	})

	rules = append(rules, groupRules...)

	return rules
}

// Provider implements authn.User.
func (u *User) UserProvider() string {
	return u.Provider
}

// Subject implements authn.User.
func (u *User) UserSubject() string {
	return u.Subject
}

var _ authz.User = &User{}

func (s *Store) FindOrCreateUser(ctx context.Context, subject, provider string) (*User, error) {
	var user *User
	err := s.Tx(ctx, func(conn *sqlite.Conn) error {
		query := fmt.Sprintf(`SELECT %s FROM users WHERE subject = ? AND provider = ? LIMIT 1`, userAttributes)
		err := sqlitex.Execute(conn, query, &sqlitex.ExecOptions{
			Args: []any{subject, provider},
			ResultFunc: func(stmt *sqlite.Stmt) error {
				user = &User{}
				return errors.WithStack(s.bindUser(stmt, user))
			},
		})
		if err != nil {
			return errors.WithStack(err)
		}

		if user != nil {
			if err := s.joinUserGroups(ctx, conn, user); err != nil {
				return errors.WithStack(err)
			}

			return nil
		}

		query = fmt.Sprintf(`
			INSERT INTO users 
				(subject, provider, created_at, updated_at) 
			VALUES (?, ?, ?, ?) RETURNING %s;`,
			userAttributes,
		)

		now := time.Now().UTC().Unix()

		err = sqlitex.Execute(conn, query, &sqlitex.ExecOptions{
			Args: []any{subject, provider, now, now},
			ResultFunc: func(stmt *sqlite.Stmt) error {
				user = &User{}
				return errors.WithStack(s.bindUser(stmt, user))
			},
		})
		if err != nil {
			return errors.WithStack(err)
		}

		if err := s.joinUserGroups(ctx, conn, user); err != nil {
			return errors.WithStack(err)
		}

		return nil
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return user, nil
}

func (s *Store) joinUserGroups(ctx context.Context, conn *sqlite.Conn, user *User) error {
	// Query to fetch groups associated with a user through the users_groups table
	query := `
		SELECT g.id, g.name, g.created_at, g.updated_at
		FROM groups g
		JOIN users_groups ug ON g.id = ug.group_id
		WHERE ug.user_id = ?
		ORDER BY g.id
	`

	user.groups = make([]*Group, 0)

	// Execute the query to fetch groups
	err := sqlitex.Execute(conn, query, &sqlitex.ExecOptions{
		Args: []any{user.ID},
		ResultFunc: func(stmt *sqlite.Stmt) error {
			// Create a group from the row
			group := &Group{
				ID:        stmt.ColumnInt64(0),
				Name:      stmt.ColumnText(1),
				CreatedAt: time.Unix(stmt.ColumnInt64(2), 0),
				UpdatedAt: time.Unix(stmt.ColumnInt64(3), 0),
				Rules:     make([]*Rule, 0),
			}

			// Fetch rules for this group
			rulesQuery := `
				SELECT id, script, sort_order, created_at, updated_at
				FROM rules
				WHERE group_id = ?
				ORDER BY sort_order
			`

			err := sqlitex.Execute(conn, rulesQuery, &sqlitex.ExecOptions{
				Args: []any{group.ID},
				ResultFunc: func(stmt *sqlite.Stmt) error {
					rule := &Rule{
						ID:        stmt.ColumnInt64(0),
						Script:    stmt.ColumnText(1),
						SortOrder: int(stmt.ColumnInt64(2)),
						CreatedAt: time.Unix(stmt.ColumnInt64(3), 0),
						UpdatedAt: time.Unix(stmt.ColumnInt64(4), 0),
						Group:     group,
					}

					group.Rules = append(group.Rules, rule)
					return nil
				},
			})
			if err != nil {
				return errors.WithStack(err)
			}

			// Add the group to the user's groups
			user.groups = append(user.groups, group)

			return nil
		},
	})

	return errors.WithStack(err)
}

func (s *Store) UpdateUser(ctx context.Context, user *User) (*User, error) {
	var updatedUser *User

	err := s.Tx(ctx, func(conn *sqlite.Conn) error {
		// Prepare the query to update an existing user
		query := fmt.Sprintf(`
			UPDATE users SET
				updated_at = ?
			WHERE id = ? RETURNING %s
		`, userAttributes)

		// Get current timestamp for updated_at
		now := time.Now().UTC().Unix()

		updatedAt := now
		if !user.UpdatedAt.IsZero() {
			updatedAt = user.UpdatedAt.Unix()
		}

		// Execute the query
		err := sqlitex.Execute(conn, query, &sqlitex.ExecOptions{
			Args: []any{updatedAt, user.ID},
			ResultFunc: func(stmt *sqlite.Stmt) error {
				updatedUser = &User{}
				return errors.WithStack(s.bindUser(stmt, updatedUser))
			},
		})
		if err != nil {
			return errors.WithStack(err)
		}

		// Delete existing associations
		deleteQuery := `DELETE FROM users_groups WHERE user_id = ?`
		err = sqlitex.Execute(conn, deleteQuery, &sqlitex.ExecOptions{
			Args: []any{user.ID},
		})
		if err != nil {
			return errors.WithStack(err)
		}

		// Clear existing group associations and add new ones if provided
		if user.groups != nil {

			// Add new associations
			for _, group := range user.groups {
				insertQuery := `INSERT INTO users_groups (user_id, group_id) VALUES (?, ?)`
				err := sqlitex.Execute(conn, insertQuery, &sqlitex.ExecOptions{
					Args: []any{user.ID, group.ID},
				})
				if err != nil {
					return errors.WithStack(err)
				}
			}
		}

		// Join user groups
		if err := s.joinUserGroups(ctx, conn, updatedUser); err != nil {
			return errors.WithStack(err)
		}

		return nil
	})

	return updatedUser, errors.WithStack(err)
}

func (s *Store) DeleteUsers(ctx context.Context, userIDs ...int64) error {
	if len(userIDs) == 0 {
		return nil
	}

	return s.Tx(ctx, func(conn *sqlite.Conn) error {
		// Build the query with placeholders for each ID
		placeholders := make([]string, len(userIDs))
		args := make([]any, len(userIDs))

		for i, id := range userIDs {
			placeholders[i] = "?"
			args[i] = id
		}

		query := fmt.Sprintf("DELETE FROM users WHERE id IN (%s)", strings.Join(placeholders, ", "))

		// Execute the query
		return errors.WithStack(sqlitex.Execute(conn, query, &sqlitex.ExecOptions{
			Args: args,
		}))
	})
}

func (s *Store) GetUsers(ctx context.Context, userIDs ...int64) ([]*User, error) {
	var users []*User

	err := s.Do(ctx, func(conn *sqlite.Conn) error {
		var query string
		var args []any

		if len(userIDs) > 0 {
			// Build the query with placeholders for each ID
			placeholders := make([]string, len(userIDs))
			args = make([]any, len(userIDs))

			for i, id := range userIDs {
				placeholders[i] = "?"
				args[i] = id
			}

			query = fmt.Sprintf("SELECT %s FROM users WHERE id IN (%s) ORDER BY id",
				userAttributes, strings.Join(placeholders, ", "))
		} else {
			// If no IDs provided, fetch all users
			query = fmt.Sprintf("SELECT %s FROM users ORDER BY id", userAttributes)
		}

		// Execute the query
		err := sqlitex.Execute(conn, query, &sqlitex.ExecOptions{
			Args: args,
			ResultFunc: func(stmt *sqlite.Stmt) error {
				user := &User{}
				if err := s.bindUser(stmt, user); err != nil {
					return errors.WithStack(err)
				}

				users = append(users, user)
				return nil
			},
		})
		if err != nil {
			return errors.WithStack(err)
		}

		// Fetch groups for each user
		for _, user := range users {
			if err := s.joinUserGroups(ctx, conn, user); err != nil {
				return errors.WithStack(err)
			}
		}

		return nil
	})

	return users, errors.WithStack(err)
}

func (s *Store) CountUsers(ctx context.Context) (int64, error) {
	var count int64

	err := s.Do(ctx, func(conn *sqlite.Conn) error {
		return errors.WithStack(sqlitex.Execute(conn, "SELECT COUNT(*) FROM users", &sqlitex.ExecOptions{
			ResultFunc: func(stmt *sqlite.Stmt) error {
				count = stmt.ColumnInt64(0)
				return nil
			},
		}))
	})

	return count, errors.WithStack(err)
}

var userAttributes = `id, subject, provider, nickname, email, created_at, updated_at, connected_at, basic_username, basic_password, is_admin`

func (s *Store) bindUser(stmt *sqlite.Stmt, user *User) error {
	user.ID = stmt.ColumnInt64(0)
	user.Subject = stmt.ColumnText(1)
	user.Provider = stmt.ColumnText(2)
	user.Nickname = stmt.ColumnText(3)
	user.Email = stmt.ColumnText(4)
	user.CreatedAt = time.Unix(stmt.ColumnInt64(5), 0)
	user.UpdatedAt = time.Unix(stmt.ColumnInt64(6), 0)
	user.ConnectedAt = time.Unix(stmt.ColumnInt64(7), 0)
	user.BasicUsername = stmt.ColumnText(8)

	user.BasicPassword = make([]byte, stmt.ColumnLen(9))
	stmt.ColumnBytes(9, user.BasicPassword)
	user.IsAdmin = stmt.ColumnBool(10)

	return nil
}
