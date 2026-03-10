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
