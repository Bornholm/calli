package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"

	"github.com/bornholm/calli/internal/authz"
	"github.com/bornholm/calli/internal/config"
	"github.com/bornholm/calli/internal/explorer"
	"github.com/bornholm/calli/internal/setup"
	"github.com/bornholm/calli/pkg/log"
	"github.com/pkg/errors"
	"golang.org/x/net/webdav"

	wd "github.com/bornholm/calli/pkg/webdav"

	"github.com/bornholm/calli/pkg/webdav/filesystem"
	_ "github.com/bornholm/calli/pkg/webdav/filesystem/all"
)

var (
	configFile string = ""
	dumpConfig bool   = false
)

func init() {
	flag.StringVar(&configFile, "config", configFile, "configuration file")
	flag.BoolVar(&dumpConfig, "dump-config", dumpConfig, "dump default configuration file and exit")
}

func main() {
	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	conf := config.NewDefaultConfig()

	if dumpConfig {
		if err := config.Dump(os.Stdout, conf); err != nil {
			slog.ErrorContext(ctx, "could not dump config file", log.Error(errors.WithStack(err)))
			os.Exit(1)
		}

		os.Exit(0)
	}

	if configFile != "" {
		if err := config.LoadFile(configFile, conf); err != nil {
			slog.ErrorContext(ctx, "could not parse config file", log.Error(errors.WithStack(err)), slog.String("file", configFile))
			os.Exit(1)
		}
	}

	logger := slog.New(log.ContextHandler{
		Handler: slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level:     slog.Level(conf.Logger.Level),
			AddSource: true,
		}),
	})

	slog.SetDefault(logger)
	slog.SetLogLoggerLevel(slog.Level(conf.Logger.Level))

	fs, err := filesystem.New(filesystem.Type(conf.Filesystem.Type), conf.Filesystem.Options.Data)
	if err != nil {
		slog.ErrorContext(ctx, "could not create filesystem", log.Error(errors.WithStack(err)), slog.Any("type", conf.Filesystem.Type))
		os.Exit(1)
	}

	fs = authz.NewFileSystem(fs)
	fs = wd.WithLogger(fs, slog.Default())

	mux := &http.ServeMux{}

	handler := &webdav.Handler{
		FileSystem: fs,
		LockSystem: webdav.NewMemLS(),
		Prefix:     "/dav/",
		Logger: func(r *http.Request, err error) {
			ctx := log.WithAttrs(r.Context(), slog.String("method", r.Method), slog.String("path", r.URL.Path))
			slog.InfoContext(ctx, "http request")

			if err != nil && !errors.Is(err, os.ErrNotExist) {
				slog.ErrorContext(ctx, err.Error(), log.Error(err))
				return
			}
		},
	}

	users, err := setup.CreateUsersFromConfig(ctx, conf)
	if err != nil {
		slog.ErrorContext(ctx, "could not create users from config file", log.Error(errors.WithStack(err)))
		os.Exit(1)
	}

	basicAuth := authz.BasicAuth(users...)

	mux.Handle("/dav/", handler)
	mux.Handle("/", explorer.NewHandler(fs))

	server := http.Server{
		Addr:    string(conf.HTTP.Address),
		Handler: basicAuth(mux),
	}

	slog.InfoContext(ctx, "http server listening", slog.String("addr", server.Addr))

	if err := server.ListenAndServe(); err != nil {
		slog.ErrorContext(ctx, "could not listen", log.Error(errors.WithStack(err)))
		os.Exit(1)
	}
}
