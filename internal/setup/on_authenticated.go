package setup

import (
	"context"
	"net/http"
	"time"

	"github.com/bornholm/calli/internal/authn"
	"github.com/bornholm/calli/internal/authn/oauth2"
	"github.com/bornholm/calli/internal/authz"
	"github.com/bornholm/calli/internal/config"
	"github.com/davecgh/go-spew/spew"
	"github.com/pkg/errors"
	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

func NewOnAuthenticatedFromConfig(ctx context.Context, conf *config.Config) (func(r *http.Request, user authn.User) (*http.Request, error), error) {
	store, err := NewStoreFromConfig(ctx, conf)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return func(r *http.Request, user authn.User) (*http.Request, error) {
		ctx := r.Context()

		oauth2User, ok := user.(*oauth2.User)
		if !ok {
			return nil, errors.Errorf("unexpected user type '%T'", user.(any))
		}

		storeUser, err := store.FindOrCreateUser(ctx, user.UserSubject(), user.UserProvider())
		if err != nil {
			return nil, errors.WithStack(err)
		}

		store.Do(ctx, func(conn *sqlite.Conn) error {
			changed := false

			if storeUser.Email != oauth2User.Email {
				storeUser.Email = oauth2User.Email
				changed = true
			}

			if storeUser.Nickname != oauth2User.Nickname {
				storeUser.Nickname = oauth2User.Nickname
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
				err := sqlitex.Execute(conn, `UPDATE users SET email = ?, nickname = ?, is_admin = ? WHERE id = ?`, &sqlitex.ExecOptions{
					Args: []any{storeUser.Email, storeUser.Nickname, storeUser.IsAdmin, storeUser.ID},
				})
				if err != nil {
					return errors.WithStack(err)
				}
			}

			err := sqlitex.Execute(conn, `UPDATE users SET connected_at = ? WHERE id = ?`, &sqlitex.ExecOptions{
				Args: []any{time.Now().UTC().Unix(), storeUser.ID},
			})
			if err != nil {
				return errors.WithStack(err)
			}

			spew.Dump(storeUser)

			return nil
		})

		ctx = authz.WithContextUser(r.Context(), storeUser)
		return r.WithContext(ctx), nil
	}, nil
}
