package admin

import (
	"embed"
	"html/template"
	"time"

	"github.com/bornholm/calli/internal/store"
	"github.com/bornholm/calli/internal/ui"
	"github.com/dustin/go-humanize"
	"github.com/pkg/errors"
)

//go:embed templates/**
var templateFs embed.FS

var templates *template.Template

func init() {
	tmpl, err := ui.Templates(nil, templateFs)
	if err != nil {
		panic(errors.WithStack(err))
	}

	templates = tmpl
}

// UserTemplateData contains information about a user
type UserTemplateData struct {
	ID               int64
	Provider         string
	Subject          string
	Nickname         string
	Email            string
	IsAdmin          bool
	CreatedAt        time.Time
	UpdatedAt        time.Time
	ConnectedAt      time.Time
	HumanCreatedAt   string
	HumanUpdatedAt   string
	HumanConnectedAt string
	BasicUsername    string
}

// GroupTemplateData contains information about a group
type GroupTemplateData struct {
	ID             int64
	Name           string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	HumanCreatedAt string
	HumanUpdatedAt string
	RuleCount      int
}

// RuleTemplateData contains information about a rule
type RuleTemplateData struct {
	ID             int64
	Script         string
	SortOrder      int
	GroupID        int64
	GroupName      string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	HumanCreatedAt string
	HumanUpdatedAt string
}

// AdminDashboardTemplateData contains the data needed to render the admin dashboard
type AdminDashboardTemplateData struct {
	ui.HeadTemplateData
	ui.NavbarTemplateData
	Username   string
	IsAdmin    bool
	UserCount  int
	GroupCount int
	RuleCount  int
	Path       string
}

// UsersTemplateData contains the data needed to render the users page
type UsersTemplateData struct {
	ui.HeadTemplateData
	ui.NavbarTemplateData
	Username string
	IsAdmin  bool
	Users    []UserTemplateData
	Path     string
}

// UserFormTemplateData contains the data needed to render the user create/edit form
type UserFormTemplateData struct {
	ui.HeadTemplateData
	ui.NavbarTemplateData
	Username       string
	IsAdmin        bool
	User           UserTemplateData
	IsEdit         bool
	FormAction     string
	FormTitle      string
	SubmitBtnText  string
	ErrorMessage   string
	SuccessMessage string
	Path           string
	Groups         []GroupTemplateData
	SelectedGroups []int64
}

// UserDeleteTemplateData contains the data needed to render the user delete confirmation
type UserDeleteTemplateData struct {
	ui.HeadTemplateData
	ui.NavbarTemplateData
	Username     string
	IsAdmin      bool
	User         UserTemplateData
	Path         string
	ErrorMessage string
}

// GroupsTemplateData contains the data needed to render the groups page
type GroupsTemplateData struct {
	ui.HeadTemplateData
	ui.NavbarTemplateData
	Username string
	IsAdmin  bool
	Groups   []GroupTemplateData
	Path     string
}

// RulesTemplateData contains the data needed to render the rules page
type RulesTemplateData struct {
	ui.HeadTemplateData
	ui.NavbarTemplateData
	Username string
	IsAdmin  bool
	Rules    []RuleTemplateData
	Path     string
}

// NewUserTemplateData creates a new user template data from a store.User
func NewUserTemplateData(user *store.User) UserTemplateData {
	return UserTemplateData{
		ID:               user.ID,
		Provider:         user.Provider,
		Subject:          user.Subject,
		Nickname:         user.Nickname,
		Email:            user.Email,
		IsAdmin:          user.IsAdmin,
		CreatedAt:        user.CreatedAt,
		UpdatedAt:        user.UpdatedAt,
		ConnectedAt:      user.ConnectedAt,
		HumanCreatedAt:   humanize.Time(user.CreatedAt),
		HumanUpdatedAt:   humanize.Time(user.UpdatedAt),
		HumanConnectedAt: humanize.Time(user.ConnectedAt),
		BasicUsername:    user.BasicUsername,
	}
}

// NewGroupTemplateData creates a new group template data from a store.Group
func NewGroupTemplateData(group *store.Group) GroupTemplateData {
	return GroupTemplateData{
		ID:             group.ID,
		Name:           group.Name,
		CreatedAt:      group.CreatedAt,
		UpdatedAt:      group.UpdatedAt,
		HumanCreatedAt: humanize.Time(group.CreatedAt),
		HumanUpdatedAt: humanize.Time(group.UpdatedAt),
		RuleCount:      len(group.Rules),
	}
}

// NewRuleTemplateData creates a new rule template data from a store.Rule
func NewRuleTemplateData(rule *store.Rule) RuleTemplateData {
	groupName := ""
	groupID := int64(0)

	if rule.Group != nil {
		groupName = rule.Group.Name
		groupID = rule.Group.ID
	}

	return RuleTemplateData{
		ID:             rule.ID,
		Script:         rule.Script,
		SortOrder:      rule.SortOrder,
		GroupID:        groupID,
		GroupName:      groupName,
		CreatedAt:      rule.CreatedAt,
		UpdatedAt:      rule.UpdatedAt,
		HumanCreatedAt: humanize.Time(rule.CreatedAt),
		HumanUpdatedAt: humanize.Time(rule.UpdatedAt),
	}
}
