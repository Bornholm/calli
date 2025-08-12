package admin

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/bornholm/calli/internal/authz"
	"github.com/bornholm/calli/internal/store"
	"github.com/bornholm/calli/internal/ui"
	"github.com/bornholm/calli/pkg/log"
	"github.com/pkg/errors"
	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

// serveIndex handles requests for the admin dashboard
func (h *Handler) serveIndex(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get current authenticated user from context
	authUser, err := authz.ContextUser(ctx)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	storeUser, ok := authUser.(*store.User)
	if !ok || !storeUser.IsAdmin {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Get stats from store
	data := h.getDashboardData(ctx, storeUser)

	// Render template
	if err := templates.ExecuteTemplate(w, "index", data); err != nil {
		slog.ErrorContext(ctx, "could not execute template", log.Error(errors.WithStack(err)))
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
}

// serveGroups handles requests for the groups page
func (h *Handler) serveGroups(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get current authenticated user from context
	authUser, err := authz.ContextUser(ctx)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	storeUser, ok := authUser.(*store.User)
	if !ok || !storeUser.IsAdmin {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Get groups from store
	data := h.getGroupsData(ctx, storeUser)

	// Render template
	if err := templates.ExecuteTemplate(w, "index", data); err != nil {
		slog.ErrorContext(ctx, "could not execute template", log.Error(errors.WithStack(err)))
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
}

// getDashboardData creates template data for the admin dashboard
func (h *Handler) getDashboardData(ctx context.Context, user *store.User) AdminDashboardTemplateData {
	data := AdminDashboardTemplateData{
		HeadTemplateData: ui.HeadTemplateData{
			PageTitle: "Admin Dashboard",
		},
		NavbarTemplateData: ui.NavbarTemplateData{
			NavbarItems: []ui.NavbarItem{ui.NavbarItemLogout},
		},
		Username:   getUserDisplayName(user),
		IsAdmin:    user.IsAdmin,
		UserCount:  0,
		GroupCount: 0,
		RuleCount:  0,
		Path:       "index",
	}

	// Count users using the CountUsers method
	userCount, err := h.store.CountUsers(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "could not count users", log.Error(errors.WithStack(err)))
	} else {
		data.UserCount = int(userCount)
	}

	// Count groups and rules (still using direct SQL queries as we haven't implemented CountGroups and CountRules)
	_ = h.store.Do(ctx, func(conn *sqlite.Conn) error {
		// Count groups
		err := sqlitex.Execute(conn, "SELECT COUNT(*) FROM groups", &sqlitex.ExecOptions{
			ResultFunc: func(stmt *sqlite.Stmt) error {
				if stmt.ColumnCount() > 0 {
					data.GroupCount = int(stmt.ColumnInt64(0))
				}
				return nil
			},
		})
		if err != nil {
			slog.ErrorContext(ctx, "could not count groups", log.Error(errors.WithStack(err)))
		}

		// Count rules
		err = sqlitex.Execute(conn, "SELECT COUNT(*) FROM rules", &sqlitex.ExecOptions{
			ResultFunc: func(stmt *sqlite.Stmt) error {
				if stmt.ColumnCount() > 0 {
					data.RuleCount = int(stmt.ColumnInt64(0))
				}
				return nil
			},
		})
		if err != nil {
			slog.ErrorContext(ctx, "could not count rules", log.Error(errors.WithStack(err)))
		}

		return nil
	})

	return data
}

// getGroupsData creates template data for the groups page
func (h *Handler) getGroupsData(ctx context.Context, user *store.User) GroupsTemplateData {
	data := GroupsTemplateData{
		HeadTemplateData: ui.HeadTemplateData{
			PageTitle: "Groups - Admin",
		},
		NavbarTemplateData: ui.NavbarTemplateData{
			NavbarItems: []ui.NavbarItem{ui.NavbarItemLogout},
		},
		Username: getUserDisplayName(user),
		IsAdmin:  user.IsAdmin,
		Groups:   []GroupTemplateData{},
		Path:     "groups",
	}

	// Get groups from store
	_ = h.store.Do(ctx, func(conn *sqlite.Conn) error {
		query := "SELECT g.id, g.name, g.created_at, g.updated_at, " +
			"(SELECT COUNT(*) FROM rules r WHERE r.group_id = g.id) AS rule_count " +
			"FROM groups g ORDER BY g.id"

		err := sqlitex.Execute(conn, query, &sqlitex.ExecOptions{
			ResultFunc: func(stmt *sqlite.Stmt) error {
				// Create group from row
				group := &store.Group{
					ID:        stmt.ColumnInt64(0),
					Name:      stmt.ColumnText(1),
					CreatedAt: sqliteTimeToTime(stmt.ColumnInt64(2)),
					UpdatedAt: sqliteTimeToTime(stmt.ColumnInt64(3)),
				}

				// Create template data
				groupData := NewGroupTemplateData(group)
				groupData.RuleCount = int(stmt.ColumnInt64(4))

				// Add to template data
				data.Groups = append(data.Groups, groupData)
				return nil
			},
		})
		if err != nil {
			slog.ErrorContext(ctx, "could not get groups", log.Error(errors.WithStack(err)))
		}

		return nil
	})

	return data
}

// Helper function to get user display name
func getUserDisplayName(user *store.User) string {
	if user.Nickname != "" {
		return user.Nickname
	}
	if user.Email != "" {
		return user.Email
	}
	return "Admin"
}

// Helper function to convert SQLite timestamp to Go time
func sqliteTimeToTime(timestamp int64) time.Time {
	if timestamp == 0 {
		return time.Time{}
	}
	return time.Unix(timestamp, 0)
}
