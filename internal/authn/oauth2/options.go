package oauth2

type Options struct {
	Providers          []Provider
	SessionName        string
	Prefix             string
	PostLoginRedirect  string
	PostLogoutRedirect string
}

type OptionFunc func(opts *Options)

func NewOptions(funcs ...OptionFunc) *Options {
	opts := &Options{
		Providers:          make([]Provider, 0),
		SessionName:        "calli_auth",
		Prefix:             "",
		PostLoginRedirect:  "/",
		PostLogoutRedirect: "/",
	}

	for _, fn := range funcs {
		fn(opts)
	}

	return opts
}

func WithProviders(providers ...Provider) OptionFunc {
	return func(opts *Options) {
		opts.Providers = providers
	}
}

func WithSessionName(sessionName string) OptionFunc {
	return func(opts *Options) {
		opts.SessionName = sessionName
	}
}

func WithPrefix(prefix string) OptionFunc {
	return func(opts *Options) {
		opts.Prefix = prefix
	}
}

func WithPostLoginRedirect(path string) OptionFunc {
	return func(opts *Options) {
		opts.PostLoginRedirect = path
	}
}
func WithPostLogoutRedirect(path string) OptionFunc {
	return func(opts *Options) {
		opts.PostLogoutRedirect = path
	}
}
