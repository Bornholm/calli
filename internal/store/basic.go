package store

import (
	"context"
	"fmt"
	"math/rand/v2"

	"github.com/bornholm/calli/internal/authn"
	"github.com/bornholm/calli/internal/authn/basic"
	"github.com/pkg/errors"
	"golang.org/x/crypto/bcrypt"
	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

func (s *Store) RegenerateBasicPassword(ctx context.Context, userID int64, passwordLength int) (string, error) {
	password := generatePassword(passwordLength)

	passwordHash, err := hashPassword(password)
	if err != nil {
		return "", errors.WithStack(err)
	}

	err = s.Tx(ctx, func(conn *sqlite.Conn) error {
		query := "UPDATE users SET basic_password = ? WHERE id = ?"
		err := sqlitex.Execute(conn, query, &sqlitex.ExecOptions{
			Args: []any{passwordHash, userID},
		})
		if err != nil {
			return errors.WithStack(err)
		}

		return nil
	})
	if err != nil {
		return "", errors.WithStack(err)
	}

	return password, nil
}

// Authenticate implements authn.UserProvider.
func (s *Store) Authenticate(ctx context.Context, username string, password string) (authn.User, error) {
	var user *User
	err := s.Tx(ctx, func(conn *sqlite.Conn) error {
		query := fmt.Sprintf("SELECT %s FROM users WHERE basic_username = ? LIMIT 1", userAttributes)
		err := sqlitex.Execute(conn, query, &sqlitex.ExecOptions{
			Args: []any{username},
			ResultFunc: func(stmt *sqlite.Stmt) error {
				user = &User{}
				return errors.WithStack(s.bindUser(stmt, user))
			},
		})
		if err != nil {
			return errors.WithStack(err)
		}

		if user == nil || !verifyPassword([]byte(password), user.BasicPassword) {
			return errors.WithStack(authn.ErrUnauthenticated)
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

var _ basic.UserProvider = &Store{}

func hashPassword(password string) ([]byte, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 14)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return bytes, err
}

func verifyPassword(password, hash []byte) bool {
	err := bcrypt.CompareHashAndPassword(hash, password)
	return err == nil
}

func generatePassword(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	var password []byte
	var charSource string

	charSource += charset

	for i := 0; i < length; i++ {
		randNum := rand.IntN(len(charSource))
		password = append(password, charSource[randNum])
	}

	return string(password)
}
