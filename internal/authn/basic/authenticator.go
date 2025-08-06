package basic

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/bornholm/calli/internal/authn"
	"github.com/bornholm/calli/pkg/log"
	"github.com/pkg/errors"
)

type UserProvider interface {
	Authenticate(ctx context.Context, username, password string) (authn.User, error)
}

type UserProviderFunc func(username, password string) (authn.User, error)

func (fn UserProviderFunc) Authenticate(username, password string) (authn.User, error) {
	return fn(username, password)
}

func NewAuthenticator(userProvider UserProvider) authn.Authenticator {
	return authn.AuthenticateFunc(func(w http.ResponseWriter, r *http.Request) (authn.User, error) {
		ctx := r.Context()
		username, password, ok := r.BasicAuth()
		if ok {
			user, err := userProvider.Authenticate(ctx, username, password)
			if err != nil {
				slog.ErrorContext(ctx, "could not authenticate user", log.Error(errors.WithStack(err)))
			}

			if user != nil {
				return user, nil
			}
		}

		w.Header().Set("WWW-Authenticate", `Basic realm="restricted", charset="UTF-8"`)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)

		return nil, errors.WithStack(authn.ErrCancel)
	})
}
