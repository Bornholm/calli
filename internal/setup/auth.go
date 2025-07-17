package setup

import (
	"context"
	"slices"

	"github.com/bornholm/calli/internal/authz"
	"github.com/bornholm/calli/internal/authz/expr"
	"github.com/bornholm/calli/internal/config"
	"github.com/pkg/errors"
)

func CreateUsersFromConfig(ctx context.Context, conf *config.Config) ([]*authz.User, error) {
	groups := make([]*authz.Group, 0, len(conf.Auth.Groups))
	for _, g := range conf.Auth.Groups {
		rules := make([]authz.Rule, 0, len(*g.Rules))
		for _, r := range *g.Rules {
			rules = append(rules, expr.NewRule(r))
		}

		groups = append(groups, authz.NewGroup(string(g.Name), rules...))
	}

	users := make([]*authz.User, 0, len(conf.Auth.Users))
	for _, u := range conf.Auth.Users {
		userGroups := make([]*authz.Group, 0, len(*u.Groups))
		for _, g := range *u.Groups {
			groupIndex := slices.IndexFunc(groups, func(group *authz.Group) bool {
				if group.Name() == g {
					return true
				}

				return false
			})

			if groupIndex == -1 {
				return nil, errors.Errorf("could not find group '%s' referenced by user '%s'", g, u.Name)
			}

			userGroups = append(userGroups, groups[groupIndex])
		}

		rules := make([]authz.Rule, 0, len(*u.Rules))
		for _, r := range *u.Rules {
			rules = append(rules, expr.NewRule(r))
		}

		users = append(users, authz.NewUser(string(u.Name), string(u.Password), userGroups, rules...))
	}

	return users, nil
}
