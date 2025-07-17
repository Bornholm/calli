package authz

import "slices"

type User struct {
	name     string
	password string

	rules  []Rule
	groups []*Group
}

// Rules implements Rules.
func (u *User) Rules() []Rule {
	return slices.Collect(func(yield func(Rule) bool) {
		for _, r := range u.rules {
			if !yield(r) {
				return
			}
		}
		for _, g := range u.groups {
			for _, r := range g.Rules() {
				if !yield(r) {
					return
				}
			}
		}
	})
}

func (u *User) Name() string {
	return u.name
}

func (u *User) Password() string {
	return u.password
}

func (u *User) Groups() []*Group {
	return u.groups
}

var _ Rules = &User{}

func NewUser(name string, password string, groups []*Group, rules ...Rule) *User {
	return &User{
		name:     name,
		password: password,
		groups:   groups,
		rules:    rules,
	}
}

type Group struct {
	name  string
	rules []Rule
}

func (g *Group) Name() string {
	return g.name
}

// Rules implements Rules.
func (g *Group) Rules() []Rule {
	return g.rules
}

func NewGroup(name string, rules ...Rule) *Group {
	return &Group{name, rules}
}

var _ Rules = &Group{}
