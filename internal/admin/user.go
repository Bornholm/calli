package admin

import (
	"context"
	"fmt"
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

// serveUsers handles requests for the users page
func (h *Handler) serveUsers(w http.ResponseWriter, r *http.Request) {
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

	// Get users from store
	data := h.getUsersData(ctx, storeUser)

	// Render template
	if err := templates.ExecuteTemplate(w, "index", data); err != nil {
		slog.ErrorContext(ctx, "could not execute template", log.Error(errors.WithStack(err)))
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
}

// serveEditUser handles requests for the edit user form
func (h *Handler) serveEditUser(w http.ResponseWriter, r *http.Request) {
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

	// Get user ID from URL
	path := r.URL.Path
	var userID int64
	_, err = fmt.Sscanf(path, h.prefix+"/users/%d/edit", &userID)
	if err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	// Get user from database
	users, err := h.store.GetUsers(ctx, userID)
	if err != nil || len(users) == 0 {
		slog.ErrorContext(ctx, "could not get user", log.Error(errors.WithStack(err)))
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}
	user := users[0]

	// Get available groups
	availableGroups := h.getAllGroups(ctx)

	// Get selected group IDs from user's authz.Groups
	selectedGroups := make([]int64, 0)
	for _, authzGroup := range user.FileSystemGroups() {
		// Extract ID from group name if possible (assuming name format includes ID)
		for _, group := range availableGroups {
			if group.Name == authzGroup.Name() {
				selectedGroups = append(selectedGroups, group.ID)
				break
			}
		}
	}

	// Create form data
	data := UserFormTemplateData{
		HeadTemplateData: ui.HeadTemplateData{
			PageTitle: "Edit User - Admin",
		},
		NavbarTemplateData: ui.NavbarTemplateData{
			NavbarItems: []ui.NavbarItem{ui.NavbarItemLogout},
		},
		Username:       getUserDisplayName(storeUser),
		IsAdmin:        storeUser.IsAdmin,
		User:           NewUserTemplateData(user),
		IsEdit:         true,
		FormAction:     fmt.Sprintf("%s/users/%d/edit", h.prefix, user.ID),
		FormTitle:      "Edit User",
		SubmitBtnText:  "Update User",
		Path:           "users-edit",
		Groups:         availableGroups,
		SelectedGroups: selectedGroups,
	}

	// Render template
	if err := templates.ExecuteTemplate(w, "index", data); err != nil {
		slog.ErrorContext(ctx, "could not execute template", log.Error(errors.WithStack(err)))
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
}

// serveUpdateUser handles POST requests to update a user
func (h *Handler) serveUpdateUser(w http.ResponseWriter, r *http.Request) {
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

	// Get user ID from URL
	path := r.URL.Path
	var userID int64
	_, err = fmt.Sscanf(path, h.prefix+"/users/%d/edit", &userID)
	if err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	// Get user from database
	users, err := h.store.GetUsers(ctx, userID)
	if err != nil || len(users) == 0 {
		slog.ErrorContext(ctx, "could not get user", log.Error(errors.WithStack(err)))
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}
	user := users[0]

	// Parse form
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	// Update user fields
	user.UpdatedAt = time.Now().UTC()

	// Update the user without groups
	_, err = h.store.UpdateUser(ctx, user)
	if err != nil {
		slog.ErrorContext(ctx, "could not update user", log.Error(errors.WithStack(err)))
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	groupIDs := r.Form["groups"]

	// Delete existing associations
	_ = h.store.Do(ctx, func(conn *sqlite.Conn) error {
		deleteQuery := `DELETE FROM users_groups WHERE user_id = ?`
		return sqlitex.Execute(conn, deleteQuery, &sqlitex.ExecOptions{
			Args: []any{user.ID},
		})
	})

	// Now handle group associations through a separate query
	if len(groupIDs) > 0 {
		// Add new associations
		_ = h.store.Do(ctx, func(conn *sqlite.Conn) error {
			for _, groupIDStr := range groupIDs {
				var groupID int64
				_, err := fmt.Sscanf(groupIDStr, "%d", &groupID)
				if err == nil {
					insertQuery := `INSERT INTO users_groups (user_id, group_id) VALUES (?, ?)`
					err := sqlitex.Execute(conn, insertQuery, &sqlitex.ExecOptions{
						Args: []any{user.ID, groupID},
					})
					if err != nil {
						slog.ErrorContext(ctx, "could not add group to user", log.Error(errors.WithStack(err)))
					}
				}
			}
			return nil
		})
	}

	// Redirect to users list
	http.Redirect(w, r, fmt.Sprintf("%s/users/%d/edit", h.prefix, user.ID), http.StatusSeeOther)
}

// serveDeleteUser handles requests for the delete user confirmation
func (h *Handler) serveDeleteUser(w http.ResponseWriter, r *http.Request) {
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

	// Get user ID from URL
	path := r.URL.Path
	var userID int64
	_, err = fmt.Sscanf(path, h.prefix+"/users/%d/delete", &userID)
	if err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	// Get user from database
	users, err := h.store.GetUsers(ctx, userID)
	if err != nil || len(users) == 0 {
		slog.ErrorContext(ctx, "could not get user", log.Error(errors.WithStack(err)))
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}
	user := users[0]

	// Prevent deleting own account
	if user.ID == storeUser.ID {
		http.Error(w, "Cannot delete your own account", http.StatusBadRequest)
		return
	}

	// Create delete confirmation data
	data := UserDeleteTemplateData{
		HeadTemplateData: ui.HeadTemplateData{
			PageTitle: "Delete User - Admin",
		},
		NavbarTemplateData: ui.NavbarTemplateData{
			NavbarItems: []ui.NavbarItem{ui.NavbarItemLogout},
		},
		Username: getUserDisplayName(storeUser),
		IsAdmin:  storeUser.IsAdmin,
		User:     NewUserTemplateData(user),
		Path:     "users-delete",
	}

	// Render template
	if err := templates.ExecuteTemplate(w, "index", data); err != nil {
		slog.ErrorContext(ctx, "could not execute template", log.Error(errors.WithStack(err)))
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
}

// serveDeleteUserConfirm handles POST requests to delete a user
func (h *Handler) serveDeleteUserConfirm(w http.ResponseWriter, r *http.Request) {
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

	// Get user ID from URL
	path := r.URL.Path
	var userID int64
	_, err = fmt.Sscanf(path, h.prefix+"/users/%d/delete", &userID)
	if err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	// Get user from database
	users, err := h.store.GetUsers(ctx, userID)
	if err != nil || len(users) == 0 {
		slog.ErrorContext(ctx, "could not get user", log.Error(errors.WithStack(err)))
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}
	user := users[0]

	// Prevent deleting own account
	if user.ID == storeUser.ID {
		http.Error(w, "Cannot delete your own account", http.StatusBadRequest)
		return
	}

	// Delete user from database
	err = h.store.DeleteUsers(ctx, userID)
	if err != nil {
		slog.ErrorContext(ctx, "could not delete user", log.Error(errors.WithStack(err)))
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Redirect to users list
	http.Redirect(w, r, h.prefix+"/users", http.StatusSeeOther)
}

// getUsersData creates template data for the users page
func (h *Handler) getUsersData(ctx context.Context, user *store.User) UsersTemplateData {
	data := UsersTemplateData{
		HeadTemplateData: ui.HeadTemplateData{
			PageTitle: "Users - Admin",
		},
		NavbarTemplateData: ui.NavbarTemplateData{
			NavbarItems: []ui.NavbarItem{ui.NavbarItemLogout},
		},
		Username: getUserDisplayName(user),
		IsAdmin:  user.IsAdmin,
		Users:    []UserTemplateData{},
		Path:     "users",
	}

	// Get users from store using the GetUsers method
	users, err := h.store.GetUsers(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "could not get users", log.Error(errors.WithStack(err)))
	} else {
		// Convert store users to template data
		for _, storeUser := range users {
			data.Users = append(data.Users, NewUserTemplateData(storeUser))
		}
	}

	return data
}

// getAllGroups gets all groups from the database
func (h *Handler) getAllGroups(ctx context.Context) []GroupTemplateData {
	groups := make([]GroupTemplateData, 0)

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
				groups = append(groups, groupData)
				return nil
			},
		})
		if err != nil {
			slog.ErrorContext(ctx, "could not get groups", log.Error(errors.WithStack(err)))
		}

		return nil
	})

	return groups
}
