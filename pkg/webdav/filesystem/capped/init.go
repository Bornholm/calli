package capped

import (
	"github.com/bornholm/calli/pkg/webdav/filesystem"
	"github.com/go-viper/mapstructure/v2"
	"github.com/pkg/errors"
	"golang.org/x/net/webdav"
)

const (
	Type filesystem.Type = "capped"
)

func init() {
	filesystem.Register(Type, CreateFileSystemFromOptions)
}

type Options struct {
	MaxSize int64             `mapstructure:"maxSize"`
	Backend FileSystemOptions `mapstructure:"backend"`
}

type FileSystemOptions struct {
	Type    filesystem.Type `mapstructure:"type"`
	Options any             `mapstructure:"options"`
}

func CreateFileSystemFromOptions(options any) (webdav.FileSystem, error) {
	opts := Options{}

	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Metadata:   nil,
		DecodeHook: mapstructure.ComposeDecodeHookFunc(mapstructure.StringToTimeDurationHookFunc()),
		Result:     &opts,
	})
	if err != nil {
		return nil, errors.Wrapf(err, "could not create '%s' filesystem options decoder", Type)
	}

	if err := decoder.Decode(options); err != nil {
		return nil, errors.Wrapf(err, "could not parse '%s' filesystem options", Type)
	}

	backend, err := filesystem.New(opts.Backend.Type, opts.Backend.Options)
	if err != nil {
		return nil, errors.Wrapf(err, "could not create backend filesystem '%s'", opts.Backend.Type)
	}

	fs := NewFileSystem(backend, opts.MaxSize)

	return fs, nil
}
