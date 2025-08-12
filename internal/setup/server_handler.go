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
	"github.com/bornholm/calli/pkg/log"
	"github.com/bornholm/calli/pkg/webdav/filesystem"
	"github.com/pkg/errors"
	"golang.org/x/net/webdav"

	wd "github.com/bornholm/calli/pkg/webdav"
)

func NewHandlerFromConfig(ctx context.Context, conf *config.Config) (http.Handler, error) {
	mux := &http.ServeMux{}

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
			ctx := r.Context()
			ctx = log.WithAttrs(ctx, slog.String("method", r.Method), slog.String("path", r.URL.Path))
			slog.InfoContext(ctx, "http request")

			if err != nil && !errors.Is(err, os.ErrNotExist) {
				slog.ErrorContext(ctx, err.Error(), log.Error(err))
				return
			}
		},
	}

	oauth2Handler, err := NewOAuth2HandlerFromConfig(ctx, conf)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	mux.Handle("/auth/", oauth2Handler)

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

	mux.Handle("/dav/", davAuth(davHandler))

	uiAuth := authn.Chain(
		authn.WithAuthenticators(
			oauth2Handler.Authenticator(true),
		),
		authn.WithOnAuthenticated(onAuthenticated),
	)

	// Explorer handler with store for credential regeneration
	mux.Handle("/", uiAuth(explorer.NewHandler(string(conf.HTTP.BaseURL), fs, store)))

	adminHandler := admin.NewHandler("/admin", store)
	mux.Handle("/admin/", uiAuth(adminHandler))

	return mux, nil
}
