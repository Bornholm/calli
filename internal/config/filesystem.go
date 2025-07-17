package config

import (
	"fmt"
	"slices"
	"strings"

	"github.com/bornholm/calli/pkg/webdav/filesystem"
	"github.com/bornholm/calli/pkg/webdav/filesystem/local"
	"github.com/bornholm/calli/pkg/webdav/filesystem/s3"
	"github.com/goccy/go-yaml"
	"github.com/pkg/errors"
)

type Filesystem struct {
	Type    InterpolatedString `yaml:"type"`
	Options *InterpolatedMap   `yaml:"options"`
}

func NewDefaultFilesystemConfig() Filesystem {
	return Filesystem{
		Type: InterpolatedString(fmt.Sprintf("${CALLI_FILESYSTEM_TYPE:-%s}", local.Type)),
		Options: &InterpolatedMap{
			Data: map[string]any{
				"dir": "${CALLI_FILESYSTEM_DIR:-./data}",
			},
		},
	}
}

func NewFilesystemConfigCommentMap() yaml.CommentMap {
	return yaml.CommentMap{
		"":      []*yaml.Comment{yaml.HeadComment(" Filesystem configuration")},
		".type": []*yaml.Comment{yaml.HeadComment(" Filesystem type", fmt.Sprintf(" Available: %v", filesystem.Registered()))},
		".options": []*yaml.Comment{
			yaml.HeadComment(" Filesystem options"),
			getFilesystemOptionComment("S3 filesystem", s3.Options{}),
		},
	}
}

func getFilesystemOptionComment(message string, opts any) *yaml.Comment {
	rawOpts, err := yaml.Marshal(opts)
	if err != nil {
		panic(errors.WithStack(err))
	}

	comments := []string{message, "options:"}
	comments = append(comments, slices.Collect(func(yield func(string) bool) {
		for _, str := range strings.Split(string(rawOpts), "\n") {
			if !yield("  " + str) {
				return
			}
		}
	})...)

	return yaml.FootComment(comments...)
}
