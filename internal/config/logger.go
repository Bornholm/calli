package config

import (
	"log/slog"

	"github.com/goccy/go-yaml"
)

type Logger struct {
	Level InterpolatedInt `yaml:"level"`
}

func NewDefaultLoggerConfig() Logger {
	return Logger{
		Level: InterpolatedInt(slog.LevelInfo),
	}
}

func NewLoggerConfigCommentMap() yaml.CommentMap {
	return yaml.CommentMap{
		"":       []*yaml.Comment{yaml.HeadComment(" Logger configuration")},
		".level": []*yaml.Comment{yaml.HeadComment(" Logging level (debug: -4, info: 0, warn: 4, error: 8)")},
	}
}
