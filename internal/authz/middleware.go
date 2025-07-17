package authz

import (
	"crypto/sha256"
	"crypto/subtle"
	"log/slog"
	"net/http"
	"slices"

	"github.com/bornholm/calli/pkg/log"
)

func BasicAuth(users ...*User) func(h http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		var fn http.HandlerFunc = func(w http.ResponseWriter, r *http.Request) {
			username, password, ok := r.BasicAuth()
			if ok {
				usernameHash := sha256.Sum256([]byte(username))
				passwordHash := sha256.Sum256([]byte(password))

				userIndex := slices.IndexFunc(users, func(u *User) bool {
					return u.Name() == username
				})

				if userIndex != -1 {
					user := users[userIndex]

					expectedUsername := sha256.Sum256([]byte(user.Name()))
					expectedPassword := sha256.Sum256([]byte(user.Password()))

					usernameMatch := (subtle.ConstantTimeCompare(usernameHash[:], expectedUsername[:]) == 1)
					passwordMatch := (subtle.ConstantTimeCompare(passwordHash[:], expectedPassword[:]) == 1)

					if usernameMatch && passwordMatch {
						ctx := WithContextUser(r.Context(), user)
						ctx = log.WithAttrs(ctx, slog.String("user", user.Name()))

						next.ServeHTTP(w, r.WithContext(ctx))
						return
					}
				}
			}

			w.Header().Set("WWW-Authenticate", `Basic realm="restricted", charset="UTF-8"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
		}
		return fn
	}
}
