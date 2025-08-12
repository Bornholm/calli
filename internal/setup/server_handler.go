package setup

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	"github.com/bornholm/calli/internal/admin"
	"github.com/bornholm/calli/internal/authn"
	"github.com/bornholm/calli/internal/authn/basic"
	"github.com/bornholm/calli/internal/authz"
	"github.com/bornholm/calli/internal/config"
	"github.com/bornholm/calli/internal/explorer"
	"github.com/bornholm/calli/internal/ratelimit"
	"github.com/bornholm/calli/pkg/log"
	"github.com/bornholm/calli/pkg/webdav/filesystem"
	"github.com/pkg/errors"
	"golang.org/x/net/webdav"

	wd "github.com/bornholm/calli/pkg/webdav"

	sloghttp "github.com/samber/slog-http"
)

func NewHandlerFromConfig(ctx context.Context, conf *config.Config) (http.Handler, error) {
	mux := &http.ServeMux{}

	slogMiddleware := sloghttp.New(slog.Default())

	fs, err := filesystem.New(filesystem.Type(conf.Filesystem.Type), conf.Filesystem.Options.Data)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	fs = authz.NewFileSystem(fs)
	fs = wd.WithLogger(fs, slog.Default())

	davHandler := &webdav.Handler{
		FileSystem: fs,
		LockSystem: webdav.NewMemLS(),
		Prefix:     "/dav/",
		Logger: func(r *http.Request, err error) {
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				ctx := r.Context()
				slog.ErrorContext(ctx, err.Error(), log.Error(err))
				return
			}
		},
	}

	oauth2Handler, err := NewOAuth2HandlerFromConfig(ctx, conf)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	mux.Handle("/auth/", slogMiddleware(oauth2Handler))

	store, err := NewStoreFromConfig(ctx, conf)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	onAuthenticated, err := NewOnAuthenticatedFromConfig(ctx, conf)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	davAuth := authn.Chain(
		authn.WithAuthenticators(
			oauth2Handler.Authenticator(false),
			basic.NewAuthenticator(store),
		),
		authn.WithOnAuthenticated(onAuthenticated),
	)

	rateLimiter := ratelimit.New(10, 20)
	rateLimiterMiddleware := rateLimiter.Middleware(func(r *http.Request) (string, error) {
		user, err := authn.ContextUser(r.Context())
		if err != nil {
			return "", errors.WithStack(err)
		}

		return user.UserProvider() + "-" + user.UserSubject(), nil
	})

	mux.Handle("/dav/", davAuth(slogMiddleware(rateLimiterMiddleware(davHandler))))

	uiAuth := authn.Chain(
		authn.WithAuthenticators(
			oauth2Handler.Authenticator(true),
		),
		authn.WithOnAuthenticated(onAuthenticated),
	)

	// Explorer handler with store for credential regeneration
	mux.Handle("/", uiAuth(slogMiddleware(explorer.NewHandler(string(conf.HTTP.BaseURL), fs, store))))

	adminHandler := admin.NewHandler("/admin", store)
	mux.Handle("/admin/", uiAuth(adminHandler))

	return mux, nil
}
