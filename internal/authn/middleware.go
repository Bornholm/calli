package authn

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/bornholm/calli/pkg/log"
	"github.com/pkg/errors"
)

var (
	ErrCancel = errors.New("cancel")
)

type Authenticator interface {
	Authenticate(w http.ResponseWriter, r *http.Request) (User, error)
}

type AuthenticateFunc func(w http.ResponseWriter, r *http.Request) (User, error)

func (fn AuthenticateFunc) Authenticate(w http.ResponseWriter, r *http.Request) (User, error) {
	return fn(w, r)
}

func Chain(funcs ...MiddlewareOptionFunc) func(http.Handler) http.Handler {
	opts := NewMiddlewareOptions(funcs...)
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			for _, auth := range opts.Authenticators {
				user, err := auth.Authenticate(w, r)
				if errors.Is(err, ErrCancel) {
					return
				}

				if user == nil {
					continue
				}

				ctx = setContextUser(ctx, user)
				ctx = log.WithAttrs(ctx, slog.String("user", fmt.Sprintf("%s@%s", user.UserSubject(), user.UserProvider())))
				r = r.WithContext(ctx)

				rr, err := opts.OnAuthenticated(r, user)
				if err != nil {
					opts.OnError(w, r, err)
					return
				}

				next.ServeHTTP(w, rr)
				return
			}

			opts.UnauthorizedHandler.ServeHTTP(w, r)
		}

		return http.HandlerFunc(fn)
	}
}

type OnAuthenticatedFunc func(r *http.Request, user User) (*http.Request, error)
type OnErrorFunc func(w http.ResponseWriter, r *http.Request, err error)

type MiddlewareOptions struct {
	UnauthorizedHandler http.Handler
	Authenticators      []Authenticator
	OnAuthenticated     OnAuthenticatedFunc
	OnError             OnErrorFunc
}

type MiddlewareOptionFunc func(opts *MiddlewareOptions)

func NewMiddlewareOptions(funcs ...MiddlewareOptionFunc) *MiddlewareOptions {
	opts := &MiddlewareOptions{
		OnAuthenticated: func(r *http.Request, user User) (*http.Request, error) {
			return r, nil
		},
		UnauthorizedHandler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		}),
		OnError: func(w http.ResponseWriter, r *http.Request, err error) {
			slog.ErrorContext(r.Context(), "authentication error", log.Error(errors.WithStack(err)))
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		},
	}

	for _, fn := range funcs {
		fn(opts)
	}

	return opts
}

func WithAuthenticators(authenticators ...Authenticator) MiddlewareOptionFunc {
	return func(opts *MiddlewareOptions) {
		opts.Authenticators = authenticators
	}
}

func WithUnauthorizedHandler(h http.Handler) MiddlewareOptionFunc {
	return func(opts *MiddlewareOptions) {
		opts.UnauthorizedHandler = h
	}
}

func WithOnAuthenticated(fn OnAuthenticatedFunc) MiddlewareOptionFunc {
	return func(opts *MiddlewareOptions) {
		opts.OnAuthenticated = fn
	}
}
