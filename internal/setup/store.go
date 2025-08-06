package setup

import (
	"context"

	"github.com/bornholm/calli/internal/config"
	"github.com/bornholm/calli/internal/store"
	"github.com/pkg/errors"
)

var NewStoreFromConfig = createFromConfigOnce(func(ctx context.Context, conf *config.Config) (*store.Store, error) {
	store := store.NewStore(string(conf.Store.Path))

	if err := store.HealthCheck(ctx); err != nil {
		return nil, errors.WithStack(err)
	}

	return store, nil
})
