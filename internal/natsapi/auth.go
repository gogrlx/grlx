package natsapi

import (
	"encoding/json"
	"fmt"

	apitypes "github.com/gogrlx/grlx/v2/internal/api/types"
	intauth "github.com/gogrlx/grlx/v2/internal/auth"
	"github.com/gogrlx/grlx/v2/internal/rbac"
)

// AuthParams holds the user's auth token for identity resolution.
type AuthParams struct {
	Token string `json:"token"`
}

func handleAuthWhoAmI(params json.RawMessage) (any, error) {
	var p AuthParams
	if len(params) > 0 {
		json.Unmarshal(params, &p)
	}

	if p.Token == "" {
		if intauth.DangerouslyAllowRoot() {
			return apitypes.UserInfo{
				Pubkey:   "(dangerously_allow_root)",
				RoleName: "admin",
			}, nil
		}
		return nil, fmt.Errorf("unauthorized")
	}

	pubkey, roleName, err := intauth.WhoAmI(p.Token)
	if err != nil {
		return nil, fmt.Errorf("unauthorized")
	}

	return apitypes.UserInfo{
		Pubkey:   pubkey,
		RoleName: roleName,
	}, nil
}

func handleAuthExplain(params json.RawMessage) (any, error) {
	var p AuthParams
	if len(params) > 0 {
		json.Unmarshal(params, &p)
	}

	policy := intauth.CurrentPolicy()

	if p.Token == "" {
		if intauth.DangerouslyAllowRoot() {
			return apitypes.ExplainResponse{
				Pubkey:   "(dangerously_allow_root)",
				RoleName: "admin",
				IsAdmin:  true,
				Actions:  []apitypes.ActionExplain{{Action: "admin", Scope: "*"}},
			}, nil
		}
		return nil, fmt.Errorf("unauthorized")
	}

	pubkey, roleName, err := intauth.WhoAmI(p.Token)
	if err != nil {
		return nil, fmt.Errorf("unauthorized")
	}

	summary := rbac.ExplainAccess(policy, pubkey)

	resp := apitypes.ExplainResponse{
		Pubkey:   pubkey,
		RoleName: roleName,
		IsAdmin:  summary.IsAdmin,
		Warnings: summary.Warnings,
	}
	for _, a := range summary.Actions {
		resp.Actions = append(resp.Actions, apitypes.ActionExplain{
			Action: a.Action,
			Scope:  a.Scope,
		})
	}
	return resp, nil
}

// UserAddParams holds the params for adding a user.
type UserAddParams struct {
	Token    string `json:"token"`
	Pubkey   string `json:"pubkey"`
	RoleName string `json:"role"`
}

// UserRemoveParams holds the params for removing a user.
type UserRemoveParams struct {
	Token  string `json:"token"`
	Pubkey string `json:"pubkey"`
}

func handleAuthAddUser(params json.RawMessage) (any, error) {
	var p UserAddParams
	if len(params) > 0 {
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, fmt.Errorf("invalid params: %w", err)
		}
	}

	if p.Pubkey == "" || p.RoleName == "" {
		return nil, fmt.Errorf("pubkey and role are required")
	}

	if err := intauth.AddUser(p.Pubkey, p.RoleName); err != nil {
		return nil, err
	}

	return apitypes.UserMutateResponse{
		Success: true,
		Message: fmt.Sprintf("user %s added with role %s", p.Pubkey, p.RoleName),
	}, nil
}

func handleAuthRemoveUser(params json.RawMessage) (any, error) {
	var p UserRemoveParams
	if len(params) > 0 {
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, fmt.Errorf("invalid params: %w", err)
		}
	}

	if p.Pubkey == "" {
		return nil, fmt.Errorf("pubkey is required")
	}

	if err := intauth.RemoveUser(p.Pubkey); err != nil {
		return nil, err
	}

	return apitypes.UserMutateResponse{
		Success: true,
		Message: fmt.Sprintf("user %s removed", p.Pubkey),
	}, nil
}

func handleAuthListUsers(_ json.RawMessage) (any, error) {
	users := intauth.ListAllUsers()

	roleNames := intauth.ListRoles()
	roles := make([]apitypes.RoleInfo, 0, len(roleNames))
	for _, name := range roleNames {
		role, err := intauth.GetRole(name)
		if err != nil {
			continue
		}
		roles = append(roles, apitypes.RoleInfo{
			Name:  role.Name,
			Rules: role.Rules,
		})
	}

	return apitypes.UsersListResponse{
		Users: users,
		Roles: roles,
	}, nil
}
