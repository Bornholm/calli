package authn

type User interface {
	UserSubject() string
	UserProvider() string
}
