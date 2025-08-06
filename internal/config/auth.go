package config

import "github.com/goccy/go-yaml"

type Auth struct {
	Providers AuthProviders `yaml:"providers"`
	Groups    []Group       `yaml:"groups"`
	Admins    []User        `yaml:"admins"`
}

type User struct {
	Email    InterpolatedString `yaml:"email"`
	Provider InterpolatedString `yaml:"provider"`
}

type Group struct {
	Name  InterpolatedString       `yaml:"name"`
	Rules *InterpolatedStringSlice `yaml:"rules"`
}

type AuthProviders struct {
	Google OAuth2Provider `yaml:"google"`
	Github OAuth2Provider `yaml:"github"`
	Gitea  GiteaProvider  `yaml:"gitea"`
	OIDC   OIDCProvider   `yaml:"oidc"`
}

type OAuth2Provider struct {
	Key    InterpolatedString      `yaml:"key"`
	Secret InterpolatedString      `yaml:"secret"`
	Scopes InterpolatedStringSlice `yaml:"scopes"`
}

type OIDCProvider struct {
	OAuth2Provider `yaml:",inline"`
	DiscoveryURL   InterpolatedString `yaml:"discoveryUrl"`
	Icon           InterpolatedString `yaml:"icon"`
	Label          InterpolatedString `yaml:"label"`
}

type GiteaProvider struct {
	OAuth2Provider `yaml:",inline"`
	TokenURL       InterpolatedString `yaml:"tokenUrl"`
	AuthURL        InterpolatedString `yaml:"authUrl"`
	ProfileURL     InterpolatedString `yaml:"profileUrl"`
	Label          InterpolatedString `yaml:"label"`
}

func NewDefaultAuth(minimal bool) Auth {
	return Auth{
		Providers: AuthProviders{},
	}
}

func NewDefaultAuthConfig() Auth {
	return Auth{
		Admins: []User{
			{
				Email:    "",
				Provider: "google",
			},
		},
		Groups: []Group{
			{
				Name: "read-only",
				Rules: &InterpolatedStringSlice{
					"operation == OP_OPEN && bitand(flag, O_WRITE) == 0",
					"operation == OP_STAT",
				},
			},
			{
				Name: "read-write",
				Rules: &InterpolatedStringSlice{
					"true",
				},
			},
		},
	}
}

func NewAuthConfigCommentMap() yaml.CommentMap {
	return yaml.CommentMap{
		"":                    []*yaml.Comment{yaml.HeadComment(" Auth configuration")},
		".admins":             []*yaml.Comment{yaml.HeadComment(" List of users with admin privileges")},
		".admins[0].email":    []*yaml.Comment{yaml.HeadComment(" Admin's email address")},
		".admins[0].provider": []*yaml.Comment{yaml.HeadComment(" Admin's identify provider (see 'providers' section)")},
		".groups":             []*yaml.Comment{yaml.HeadComment(" Authorization groups")},
		".groups[0].rules":    []*yaml.Comment{yaml.HeadComment(" Groups authorization rules", " See https://expr-lang.org/docs/language-definition")},
	}
}
