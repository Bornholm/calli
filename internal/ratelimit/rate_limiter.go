package ratelimit

import (
	"log/slog"
	"net/http"

	"github.com/bornholm/calli/internal/syncx"
	"github.com/bornholm/calli/pkg/log"
	"github.com/pkg/errors"
	"golang.org/x/time/rate"
)

type RateLimiter struct {
	rate  rate.Limit
	burst int
	users syncx.Map[string, *rate.Limiter]
}

type GetUserKeyFunc func(r *http.Request) (string, error)

func (l *RateLimiter) Middleware(getUserKey GetUserKeyFunc) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			userKey, err := getUserKey(r)
			if err != nil {
				slog.ErrorContext(ctx, "could not retrieve user key", log.Error(errors.WithStack(err)))
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			}

			limiter, _ := l.users.LoadOrStore(userKey, rate.NewLimiter(l.rate, l.burst))

			if !limiter.Allow() {
				http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func New(rate rate.Limit, burst int) *RateLimiter {
	return &RateLimiter{
		rate:  rate,
		burst: burst,
	}
}
