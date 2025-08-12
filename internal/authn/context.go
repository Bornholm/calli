package authn

import (
	"context"

	"github.com/pkg/errors"
)

type contextKey string

const contextKeyUser contextKey = "authnUser"

func ContextUser(ctx context.Context) (User, error) {
	user, ok := ctx.Value(contextKeyUser).(User)
	if !ok {
		return nil, errors.New("no user in context")
	}

	return user, nil
}

func setContextUser(ctx context.Context, user User) context.Context {
	return context.WithValue(ctx, contextKeyUser, user)
}
