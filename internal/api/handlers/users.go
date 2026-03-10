package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/gogrlx/grlx/v2/internal/auth"
	"github.com/gogrlx/grlx/v2/internal/rbac"
)

// UserInfo represents a user's identity and role as returned by WhoAmI.
type UserInfo struct {
	Pubkey string    `json:"pubkey"`
	Role   rbac.Role `json:"role"`
}

// UsersByRole maps role names to lists of public keys.
type UsersByRole map[rbac.Role][]string

// WhoAmI returns the identity and role of the authenticated user.
func WhoAmI(w http.ResponseWriter, r *http.Request) {
	token := r.Header.Get("Authorization")
	if token == "" {
		if auth.DangerouslyAllowRoot() {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(UserInfo{
				Pubkey: "(dangerously_allow_root)",
				Role:   rbac.RoleAdmin,
			})
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	pubkey, role, err := auth.WhoAmI(token)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(UserInfo{
		Pubkey: pubkey,
		Role:   role,
	})
}

// ListUsers returns all configured users grouped by role.
func ListUsers(w http.ResponseWriter, r *http.Request) {
	users := auth.ListUsers()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(users)
}
