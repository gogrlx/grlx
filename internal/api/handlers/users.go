package handlers

import (
	"encoding/json"
	"net/http"

	apitypes "github.com/gogrlx/grlx/v2/internal/api/types"
	"github.com/gogrlx/grlx/v2/internal/auth"
)

// WhoAmI returns the identity and role of the authenticated user.
func WhoAmI(w http.ResponseWriter, r *http.Request) {
	token := r.Header.Get("Authorization")
	if token == "" {
		if auth.DangerouslyAllowRoot() {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(apitypes.UserInfo{
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
	json.NewEncoder(w).Encode(apitypes.UserInfo{
		Pubkey:   pubkey,
		RoleName: roleName,
	})
}

// ListUsers returns all configured users and role definitions.
func ListUsers(w http.ResponseWriter, r *http.Request) {
	users := auth.ListAllUsers()

	roleNames := auth.ListRoles()
	roles := make([]apitypes.RoleInfo, 0, len(roleNames))
	for _, name := range roleNames {
		role, err := auth.GetRole(name)
		if err != nil {
			continue
		}
		roles = append(roles, apitypes.RoleInfo{
			Name:  role.Name,
			Rules: role.Rules,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(apitypes.UsersListResponse{
		Users: users,
		Roles: roles,
	})
}
