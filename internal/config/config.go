package config

import (
	"bytes"
	"io"
	"os"

	"github.com/goccy/go-yaml"
	"github.com/pkg/errors"
)

type Config struct {
	Logger     Logger     `yaml:"logger"`
	HTTP       HTTP       `yaml:"http"`
	Filesystem Filesystem `yaml:"filesystem"`
	Auth       Auth       `yaml:"auth"`
	Store      Store      `yaml:"store"`
}

func NewDefaultConfig() *Config {
	return &Config{
		Logger:     NewDefaultLoggerConfig(),
		HTTP:       NewDefaultHTTPConfig(),
		Filesystem: NewDefaultFilesystemConfig(),
		Auth:       NewDefaultAuthConfig(),
		Store:      NewDefaultStoreConfig(),
	}
}

func Interpolate(conf *Config) error {
	var buff bytes.Buffer

	if err := Dump(&buff, conf); err != nil {
		return errors.WithStack(err)
	}

	if err := Load(&buff, conf); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func LoadFile(path string, conf *Config) error {
	file, err := os.OpenFile(path, os.O_RDONLY, os.ModePerm)
	if err != nil {
		return errors.WithStack(err)
	}

	defer file.Close()

	if err := Load(file, conf); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func Load(r io.Reader, conf *Config) error {
	decoder := yaml.NewDecoder(r)

	if err := decoder.Decode(conf); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

var sections = map[string]yaml.CommentMap{
	"$.http":       NewHTTPConfigCommentMap(),
	"$.filesystem": NewFilesystemConfigCommentMap(),
	"$.logger":     NewLoggerConfigCommentMap(),
	"$.auth":       NewAuthConfigCommentMap(),
}

func Dump(w io.Writer, conf *Config) error {
	configComments := yaml.CommentMap{}
	for configSelector, sectionComments := range sections {
		for sectionSelector, sectionComments := range sectionComments {
			configComments[configSelector+sectionSelector] = sectionComments
		}
	}

	encoder := yaml.NewEncoder(w, yaml.WithComment(configComments))
	defer encoder.Close()

	if err := encoder.Encode(conf); err != nil {
		return errors.WithStack(err)
	}

	return nil
}
