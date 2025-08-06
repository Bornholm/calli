package config

import (
	"time"

	"github.com/goccy/go-yaml"
)

type HTTP struct {
	BaseURL InterpolatedString `yaml:"baseUrl"`
	Address InterpolatedString `yaml:"address"`
	Session Session            `yaml:"session"`
}

type Session struct {
	Keys   InterpolatedStringSlice `yaml:"keys"`
	Cookie Cookie                  `yaml:"cookie"`
}

type Cookie struct {
	Path     InterpolatedString    `yaml:"path"`
	HTTPOnly InterpolatedBool      `yaml:"httpOnly"`
	Secure   InterpolatedBool      `yaml:"secure"`
	MaxAge   *InterpolatedDuration `yaml:"maxAge"`
}

func NewDefaultHTTPConfig() HTTP {
	return HTTP{
		BaseURL: "${CALLI_HTTP_BASE_URL:-http://localhost:8081}",
		Address: "${CALLI_HTTP_ADDRESS:-:8081}",
		Session: Session{
			Cookie: Cookie{
				Path:     "/",
				HTTPOnly: true,
				Secure:   false,
				MaxAge:   NewInterpolatedDuration(time.Hour * 24),
			},
		},
	}
}

func NewHTTPConfigCommentMap() yaml.CommentMap {
	return yaml.CommentMap{
		"":         []*yaml.Comment{yaml.HeadComment(" Webserver configuration")},
		".address": []*yaml.Comment{yaml.HeadComment(" Webserver's listening address")},
	}
}
