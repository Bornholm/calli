package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"

	"github.com/bornholm/calli/internal/config"
	"github.com/bornholm/calli/internal/setup"
	"github.com/bornholm/calli/pkg/log"
	"github.com/pkg/errors"

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

	if err := config.Interpolate(conf); err != nil {
		slog.ErrorContext(ctx, "could not interpolate config file", log.Error(errors.WithStack(err)))
		os.Exit(1)
	}

	logger := slog.New(log.ContextHandler{
		Handler: slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level:     slog.Level(conf.Logger.Level),
			AddSource: true,
		}),
	})

	slog.SetDefault(logger)
	slog.SetLogLoggerLevel(slog.Level(conf.Logger.Level))

	handler, err := setup.NewHandlerFromConfig(ctx, conf)
	if err != nil {
		slog.ErrorContext(ctx, "could not generate handler from config", log.Error(errors.WithStack(err)))
		os.Exit(1)
	}

	server := http.Server{
		Addr:    string(conf.HTTP.Address),
		Handler: handler,
	}

	slog.InfoContext(ctx, "http server listening", slog.String("addr", server.Addr))

	if err := server.ListenAndServe(); err != nil {
		slog.ErrorContext(ctx, "could not listen", log.Error(errors.WithStack(err)))
		os.Exit(1)
	}
}
