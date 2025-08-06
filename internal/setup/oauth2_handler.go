package setup

import (
	"context"
	"crypto/rand"
	"fmt"
	"net/http"

	"github.com/bornholm/calli/internal/authn/oauth2"
	"github.com/bornholm/calli/internal/config"
	"github.com/gorilla/sessions"
	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
	"github.com/markbates/goth/providers/gitea"
	"github.com/markbates/goth/providers/github"
	"github.com/markbates/goth/providers/google"
	"github.com/markbates/goth/providers/openidConnect"
	"github.com/pkg/errors"
)

func NewOAuth2HandlerFromConfig(ctx context.Context, conf *config.Config) (*oauth2.Handler, error) {
	// Configure sessions store

	keyPairs := make([][]byte, 0)
	if len(conf.HTTP.Session.Keys) == 0 {
		key, err := getRandomBytes(32)
		if err != nil {
			return nil, errors.Wrap(err, "could not generate cookie signing key")
		}

		keyPairs = append(keyPairs, key)
	} else {
		for _, k := range conf.HTTP.Session.Keys {
			keyPairs = append(keyPairs, []byte(k))
		}
	}

	sessionStore := sessions.NewCookieStore(keyPairs...)

	sessionStore.MaxAge(int(*conf.HTTP.Session.Cookie.MaxAge))
	sessionStore.Options.Path = string(conf.HTTP.Session.Cookie.Path)
	sessionStore.Options.HttpOnly = bool(conf.HTTP.Session.Cookie.HTTPOnly)
	sessionStore.Options.Secure = bool(conf.HTTP.Session.Cookie.Secure)
	sessionStore.Options.SameSite = http.SameSiteLaxMode

	// Configure providers

	gothProviders := make([]goth.Provider, 0)
	providers := make([]oauth2.Provider, 0)

	if conf.Auth.Providers.Google.Key != "" && conf.Auth.Providers.Google.Secret != "" {
		googleProvider := google.New(
			string(conf.Auth.Providers.Google.Key),
			string(conf.Auth.Providers.Google.Secret),
			fmt.Sprintf("%s/auth/providers/google/callback", conf.HTTP.BaseURL),
			conf.Auth.Providers.Google.Scopes...,
		)

		gothProviders = append(gothProviders, googleProvider)

		providers = append(providers, oauth2.Provider{
			ID:    googleProvider.Name(),
			Label: "Google",
			Icon:  "fa-google",
		})
	}

	if conf.Auth.Providers.Github.Key != "" && conf.Auth.Providers.Github.Secret != "" {
		githubProvider := github.New(
			string(conf.Auth.Providers.Github.Key),
			string(conf.Auth.Providers.Github.Secret),
			fmt.Sprintf("%s/auth/providers/github/callback", conf.HTTP.BaseURL),
			conf.Auth.Providers.Github.Scopes...,
		)

		gothProviders = append(gothProviders, githubProvider)

		providers = append(providers, oauth2.Provider{
			ID:    githubProvider.Name(),
			Label: "Github",
			Icon:  "fa-github",
		})
	}

	if conf.Auth.Providers.Gitea.Key != "" && conf.Auth.Providers.Gitea.Secret != "" {
		giteaProvider := gitea.NewCustomisedURL(
			string(conf.Auth.Providers.Gitea.Key),
			string(conf.Auth.Providers.Gitea.Secret),
			fmt.Sprintf("%s/auth/providers/gitea/callback", conf.HTTP.BaseURL),
			string(conf.Auth.Providers.Gitea.AuthURL),
			string(conf.Auth.Providers.Gitea.TokenURL),
			string(conf.Auth.Providers.Gitea.ProfileURL),
			conf.Auth.Providers.Gitea.Scopes...,
		)

		gothProviders = append(gothProviders, giteaProvider)

		providers = append(providers, oauth2.Provider{
			ID:    giteaProvider.Name(),
			Label: string(conf.Auth.Providers.Gitea.Label),
			Icon:  "fa-git-alt",
		})
	}

	if conf.Auth.Providers.OIDC.Key != "" && conf.Auth.Providers.OIDC.Secret != "" {
		oidcProvider, err := openidConnect.New(
			string(conf.Auth.Providers.OIDC.Key),
			string(conf.Auth.Providers.OIDC.Secret),
			fmt.Sprintf("%s/auth/providers/openid-connect/callback", conf.HTTP.BaseURL),
			string(conf.Auth.Providers.OIDC.DiscoveryURL),
			conf.Auth.Providers.OIDC.Scopes...,
		)
		if err != nil {
			return nil, errors.Wrap(err, "could not configure oidc provider")
		}

		gothProviders = append(gothProviders, oidcProvider)

		providers = append(providers, oauth2.Provider{
			ID:    oidcProvider.Name(),
			Label: string(conf.Auth.Providers.OIDC.Label),
			Icon:  string(conf.Auth.Providers.OIDC.Icon),
		})
	}

	goth.UseProviders(gothProviders...)
	gothic.Store = sessionStore

	opts := []oauth2.OptionFunc{
		oauth2.WithProviders(providers...),
		oauth2.WithPrefix("/auth"),
	}

	auth := oauth2.NewHandler(
		sessionStore,
		opts...,
	)

	return auth, nil
}

func getRandomBytes(n int) ([]byte, error) {
	data := make([]byte, n)

	read, err := rand.Read(data)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	if read != n {
		return nil, errors.Errorf("could not read %d bytes", n)
	}

	return data, nil
}
