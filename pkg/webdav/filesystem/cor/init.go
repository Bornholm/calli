package cor

import (
	"github.com/bornholm/calli/pkg/webdav/filesystem"
	"github.com/go-viper/mapstructure/v2"
	"github.com/pkg/errors"
	"golang.org/x/net/webdav"
)

const (
	Type      filesystem.Type = "cor"
	TypeAlias filesystem.Type = "cacheonread"
)

func init() {
	filesystem.Register(Type, CreateFileSystemFromOptions)
	filesystem.Register(TypeAlias, CreateFileSystemFromOptions)
}

type Options struct {
	Cache   FileSystemOptions
	Backend FileSystemOptions
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

	cache, err := filesystem.New(opts.Cache.Type, opts.Cache.Options)
	if err != nil {
		return nil, errors.Wrapf(err, "could not create cache filesystem '%s'", opts.Cache.Type)
	}

	backend, err := filesystem.New(opts.Backend.Type, opts.Backend.Options)
	if err != nil {
		return nil, errors.Wrapf(err, "could not create backend filesystem '%s'", opts.Backend.Type)
	}

	fs := NewFileSystem(cache, backend)

	return fs, nil
}
