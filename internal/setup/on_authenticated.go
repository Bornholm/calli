package setup

import (
	"context"
	"net/http"
	"time"

	"github.com/bornholm/calli/internal/authn"
	"github.com/bornholm/calli/internal/authn/oauth2"
	"github.com/bornholm/calli/internal/authz"
	"github.com/bornholm/calli/internal/config"
	"github.com/bornholm/calli/internal/store"
	"github.com/pkg/errors"
	"github.com/rs/xid"
	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

func NewOnAuthenticatedFromConfig(ctx context.Context, conf *config.Config) (func(r *http.Request, user authn.User) (*http.Request, error), error) {
	st, err := NewStoreFromConfig(ctx, conf)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return func(r *http.Request, user authn.User) (*http.Request, error) {
		ctx := r.Context()

		var storeUser *store.User

		switch typedUser := user.(type) {
		case *oauth2.User:
			storeUser, err = findOrCreateUserFromOAuth2(ctx, conf, st, typedUser)
			if err != nil {
				return nil, errors.WithStack(err)
			}
		case *store.User:
			storeUser = typedUser
		}

		st.Do(ctx, func(conn *sqlite.Conn) error {
			err := sqlitex.Execute(conn, `UPDATE users SET connected_at = ? WHERE id = ?`, &sqlitex.ExecOptions{
				Args: []any{time.Now().UTC().Unix(), storeUser.ID},
			})
			if err != nil {
				return errors.WithStack(err)
			}

			return nil
		})
		if err != nil {
			return nil, errors.WithStack(err)
		}

		ctx = authz.WithContextUser(r.Context(), storeUser)

		return r.WithContext(ctx), nil
	}, nil
}

func findOrCreateUserFromOAuth2(ctx context.Context, conf *config.Config, st *store.Store, user *oauth2.User) (*store.User, error) {
	storeUser, err := st.FindOrCreateUser(ctx, user.UserSubject(), user.UserProvider())
	if err != nil {
		return nil, errors.WithStack(err)
	}

	err = st.Do(ctx, func(conn *sqlite.Conn) error {
		changed := false

		if storeUser.BasicUsername == "" {
			storeUser.BasicUsername = xid.New().String()
			changed = true
		}

		if storeUser.Email != user.Email {
			storeUser.Email = user.Email
			changed = true
		}

		if storeUser.Nickname != user.Nickname {
			storeUser.Nickname = user.Nickname
			changed = true
		}

		isAdmin := false
		for _, u := range conf.Auth.Admins {
			if string(u.Email) != storeUser.Email || string(u.Provider) != storeUser.Provider {
				continue
			}

			isAdmin = true
			break
		}

		if storeUser.IsAdmin != isAdmin {
			storeUser.IsAdmin = isAdmin
			changed = true
		}

		if changed {
			err := sqlitex.Execute(conn, `UPDATE users SET basic_username = ?, email = ?, nickname = ?, is_admin = ? WHERE id = ?`, &sqlitex.ExecOptions{
				Args: []any{storeUser.BasicUsername, storeUser.Email, storeUser.Nickname, storeUser.IsAdmin, storeUser.ID},
			})
			if err != nil {
				return errors.WithStack(err)
			}
		}

		return nil
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	if storeUser.BasicUsername == "" || storeUser.BasicPassword == nil {
		if _, err := st.RegenerateBasicPassword(ctx, storeUser.ID, 14); err != nil {
			return nil, errors.WithStack(err)
		}
	}

	return storeUser, nil
}
