package config

import "github.com/goccy/go-yaml"

type Auth struct {
	Users  []User  `yaml:"users"`
	Groups []Group `yaml:"groups"`
}

type User struct {
	Name     InterpolatedString       `yaml:"name"`
	Password InterpolatedString       `yaml:"password"`
	Groups   *InterpolatedStringSlice `yaml:"groups"`
	Rules    *InterpolatedStringSlice `yaml:"rules"`
}

type Group struct {
	Name  InterpolatedString       `yaml:"name"`
	Rules *InterpolatedStringSlice `yaml:"rules"`
}

func NewDefaultAuthConfig() Auth {
	return Auth{
		Users: []User{
			{
				Name:     "reader",
				Password: "reader",
				Groups:   &InterpolatedStringSlice{"read-only"},
				Rules:    &InterpolatedStringSlice{},
			},
			{
				Name:     "writer",
				Password: "writer",
				Groups:   &InterpolatedStringSlice{"read-write"},
				Rules:    &InterpolatedStringSlice{},
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
		"":                   []*yaml.Comment{yaml.HeadComment(" Auth configuration")},
		".users":             []*yaml.Comment{yaml.HeadComment(" Authorized users with their credentials")},
		".users[0].name":     []*yaml.Comment{yaml.HeadComment(" User's name")},
		".users[0].password": []*yaml.Comment{yaml.HeadComment(" User's password")},
		".users[0].rules":    []*yaml.Comment{yaml.HeadComment(" User's custom authorization rules", " See https://expr-lang.org/docs/language-definition")},
		".users[0].groups":   []*yaml.Comment{yaml.HeadComment(" User's authorization groups")},
		".groups":            []*yaml.Comment{yaml.HeadComment(" Authorization groups")},
		".groups[0].rules":   []*yaml.Comment{yaml.HeadComment(" Groups authorization rules", " See https://expr-lang.org/docs/language-definition")},
	}
}
