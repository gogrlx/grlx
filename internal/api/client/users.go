package client

import (
	"encoding/json"
	"fmt"

	apitypes "github.com/gogrlx/grlx/v2/internal/api/types"
)

// WhoAmI retrieves the identity and role of the authenticated user.
func WhoAmI() (apitypes.UserInfo, error) {
	var info apitypes.UserInfo
	resp, err := NatsRequest("auth.whoami", nil)
	if err != nil {
		return info, err
	}
	if err := json.Unmarshal(resp, &info); err != nil {
		return info, fmt.Errorf("whoami: %w", err)
	}
	return info, nil
}

// ExplainAccess retrieves a permission summary for the authenticated user.
func ExplainAccess() (apitypes.ExplainResponse, error) {
	var result apitypes.ExplainResponse
	resp, err := NatsRequest("auth.explain", nil)
	if err != nil {
		return result, err
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return result, fmt.Errorf("explain: %w", err)
	}
	return result, nil
}

// ListUsers retrieves all configured users and role definitions.
func ListUsers() (apitypes.UsersListResponse, error) {
	var result apitypes.UsersListResponse
	resp, err := NatsRequest("auth.users", nil)
	if err != nil {
		return result, err
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return result, fmt.Errorf("list users: %w", err)
	}
	return result, nil
}

// AddUser adds a pubkey→role mapping on the farmer.
func AddUser(pubkey, roleName string) (apitypes.UserMutateResponse, error) {
	var result apitypes.UserMutateResponse
	params := apitypes.UserAddRequest{
		Pubkey:   pubkey,
		RoleName: roleName,
	}
	data, err := json.Marshal(params)
	if err != nil {
		return result, fmt.Errorf("add user: %w", err)
	}
	resp, err := NatsRequest("auth.users.add", data)
	if err != nil {
		return result, err
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return result, fmt.Errorf("add user: %w", err)
	}
	return result, nil
}

// RemoveUser removes a pubkey mapping on the farmer.
func RemoveUser(pubkey string) (apitypes.UserMutateResponse, error) {
	var result apitypes.UserMutateResponse
	params := apitypes.UserRemoveRequest{
		Pubkey: pubkey,
	}
	data, err := json.Marshal(params)
	if err != nil {
		return result, fmt.Errorf("remove user: %w", err)
	}
	resp, err := NatsRequest("auth.users.remove", data)
	if err != nil {
		return result, err
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return result, fmt.Errorf("remove user: %w", err)
	}
	return result, nil
}
