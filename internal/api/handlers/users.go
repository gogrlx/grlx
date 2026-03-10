package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/gogrlx/grlx/v2/internal/auth"
	"github.com/gogrlx/grlx/v2/internal/rbac"
)

// UserInfo represents a user's identity and role as returned by WhoAmI.
type UserInfo struct {
	Pubkey   string `json:"pubkey"`
	RoleName string `json:"role"`
}

// RoleInfo describes a role and its rules.
type RoleInfo struct {
	Name  string      `json:"name"`
	Rules []rbac.Rule `json:"rules"`
}

// UsersListResponse contains all users and role definitions.
type UsersListResponse struct {
	Users map[string]string `json:"users"` // pubkey → role name
	Roles []RoleInfo        `json:"roles"`
}

// WhoAmI returns the identity and role of the authenticated user.
func WhoAmI(w http.ResponseWriter, r *http.Request) {
	token := r.Header.Get("Authorization")
	if token == "" {
		if auth.DangerouslyAllowRoot() {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(UserInfo{
				Pubkey:   "(dangerously_allow_root)",
				RoleName: "admin",
			})
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	pubkey, roleName, err := auth.WhoAmI(token)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(UserInfo{
		Pubkey:   pubkey,
		RoleName: roleName,
	})
}

// ListUsers returns all configured users and role definitions.
func ListUsers(w http.ResponseWriter, r *http.Request) {
	users := auth.ListAllUsers()

	roleNames := auth.ListRoles()
	roles := make([]RoleInfo, 0, len(roleNames))
	for _, name := range roleNames {
		role, err := auth.GetRole(name)
		if err != nil {
			continue
		}
		roles = append(roles, RoleInfo{
			Name:  role.Name,
			Rules: role.Rules,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(UsersListResponse{
		Users: users,
		Roles: roles,
	})
}
