package s3

import (
	"os"

	"github.com/bornholm/calli/pkg/webdav/filesystem"
	"github.com/go-viper/mapstructure/v2"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/pkg/errors"
	"golang.org/x/net/webdav"
)

const Type filesystem.Type = "s3"

func init() {
	filesystem.Register(Type, CreateFileSystemFromOptions)
}

type Options struct {
	Endpoint     string `mapstructure:"endpoint" yaml:"endpoint"`
	User         string `mapstructure:"user" yaml:"user"`
	Secret       string `mapstructure:"secret" yaml:"secret"`
	Token        string `mapstructure:"token" yaml:"token"`
	Secure       bool   `mapstructure:"secure" yaml:"secure"`
	Bucket       string `mapstructure:"bucket" yaml:"bucket"`
	Region       string `mapstructure:"region" yaml:"region"`
	BucketLookup string `mapstructure:"bucketLookup" yaml:"bucketLookup"`
	// Enable/disable HTTP tracing in the console
	Trace bool `mapstructure:"trace" yaml:"trace"`
}

func CreateFileSystemFromOptions(options any) (webdav.FileSystem, error) {
	opts := Options{}

	if err := mapstructure.Decode(options, &opts); err != nil {
		return nil, errors.Wrapf(err, "could not parse '%s' filesystem options", Type)
	}

	creds := credentials.NewStaticV4(opts.User, opts.Secret, opts.Token)

	minioOpts := &minio.Options{
		Creds:  creds,
		Secure: opts.Secure,
		Region: opts.Region,
	}

	switch opts.BucketLookup {
	case "dns":
		minioOpts.BucketLookup = minio.BucketLookupDNS
	case "path":
		minioOpts.BucketLookup = minio.BucketLookupPath
	default:
		return nil, errors.Errorf("unknown bucket lookup value '%s', expected 'dns' or 'path'", opts.BucketLookup)
	}

	client, err := minio.New(opts.Endpoint, minioOpts)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	if opts.Trace {
		client.TraceOn(os.Stdout)
	}

	fs := NewFileSystem(client, opts.Bucket)

	return fs, nil
}
