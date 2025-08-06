package store

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"fmt"
	"slices"
	"time"

	"github.com/bornholm/calli/internal/authn"
	"github.com/bornholm/calli/internal/authn/basic"
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
		
		webdav_username TEXT,
		webdav_password BLOB,

		UNIQUE (subject, provider),
		UNIQUE (webdav_username)
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

	WebDAVUsername string
	WebDAVPassword []byte

	groups []*Group
}

// Groups implements authz.User.
func (u *User) Groups() []*authz.Group {
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

// Rules implements authz.User.
func (u *User) Rules() []authz.Rule {
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

// Authenticate implements authn.UserProvider.
func (s *Store) Authenticate(ctx context.Context, username string, password string) (authn.User, error) {
	var user *User
	err := s.Tx(ctx, func(conn *sqlite.Conn) error {
		query := fmt.Sprintf("SELECT %s FROM users WHERE webdav_username = ? LIMIT 1", userAttributes)
		return errors.WithStack(sqlitex.Execute(conn, query, &sqlitex.ExecOptions{
			Args: []any{username},
			ResultFunc: func(stmt *sqlite.Stmt) error {
				user = &User{}
				return errors.WithStack(s.bindUser(stmt, user))
			},
		}))
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	if user == nil {
		return nil, errors.WithStack(authn.ErrUnauthenticated)
	}

	expectedUsername := sha256.Sum256([]byte(user.WebDAVUsername))

	usernameHash := sha256.Sum256([]byte(username))
	passwordHash := sha256.Sum256([]byte(password))

	usernameMatch := (subtle.ConstantTimeCompare(usernameHash[:], expectedUsername[:]) == 1)
	passwordMatch := (subtle.ConstantTimeCompare(passwordHash[:], user.WebDAVPassword[:]) == 1)

	if !usernameMatch || !passwordMatch {
		return nil, errors.WithStack(authn.ErrUnauthenticated)
	}

	err = s.Do(ctx, func(conn *sqlite.Conn) error {
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

var _ basic.UserProvider = &Store{}

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

	return nil
}

var userAttributes = `id, subject, provider, nickname, email, created_at, updated_at, connected_at, webdav_username, webdav_password, is_admin`

func (s *Store) bindUser(stmt *sqlite.Stmt, user *User) error {
	user.ID = stmt.ColumnInt64(0)
	user.Subject = stmt.ColumnText(1)
	user.Provider = stmt.ColumnText(2)
	user.Nickname = stmt.ColumnText(3)
	user.Email = stmt.ColumnText(4)
	user.CreatedAt = time.Unix(stmt.ColumnInt64(5), 0)
	user.UpdatedAt = time.Unix(stmt.ColumnInt64(6), 0)
	user.ConnectedAt = time.Unix(stmt.ColumnInt64(7), 0)
	user.WebDAVUsername = stmt.ColumnText(8)
	stmt.ColumnBytes(9, user.WebDAVPassword)
	user.IsAdmin = stmt.ColumnBool(10)

	return nil
}
