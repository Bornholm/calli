package config

import "github.com/goccy/go-yaml"

type HTTP struct {
	Address InterpolatedString `yaml:"address"`
}

func NewDefaultHTTPConfig() HTTP {
	return HTTP{
		Address: "${CALLI_HTTP_ADDRESS:-:8080}",
	}
}

func NewHTTPConfigCommentMap() yaml.CommentMap {
	return yaml.CommentMap{
		"":         []*yaml.Comment{yaml.HeadComment(" Webserver configuration")},
		".address": []*yaml.Comment{yaml.HeadComment(" Webserver's listening address")},
	}
}
