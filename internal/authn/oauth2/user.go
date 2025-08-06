package oauth2

import "github.com/bornholm/calli/internal/authn"

type User struct {
	Subject  string
	Provider string

	Nickname string
	Email    string

	AccessToken string
	IDToken     string
}

// Provider implements authn.User.
func (u *User) UserProvider() string {
	return u.Provider
}

// Subject implements authn.User.
func (u *User) UserSubject() string {
	return u.Subject
}

var _ authn.User = &User{}
