package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/gogrlx/grlx/v2/internal/api/handlers"
	"github.com/gogrlx/grlx/v2/internal/auth"
	"github.com/gogrlx/grlx/v2/internal/config"
)

// WhoAmI retrieves the identity and role of the authenticated user.
func WhoAmI() (handlers.UserInfo, error) {
	var info handlers.UserInfo
	newToken, err := auth.NewToken()
	if err != nil {
		return info, err
	}
	req, err := http.NewRequest(http.MethodGet, config.FarmerURL+"/auth/whoami", nil)
	if err != nil {
		return info, err
	}
	req.Header.Set("Authorization", newToken)

	resp, err := APIClient.Do(req)
	if err != nil {
		return info, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return info, fmt.Errorf("whoami: HTTP %d: %s", resp.StatusCode, string(body))
	}

	err = json.NewDecoder(resp.Body).Decode(&info)
	return info, err
}

// ListUsers retrieves all configured users and role definitions.
func ListUsers() (handlers.UsersListResponse, error) {
	var result handlers.UsersListResponse
	newToken, err := auth.NewToken()
	if err != nil {
		return result, err
	}
	req, err := http.NewRequest(http.MethodGet, config.FarmerURL+"/auth/users", nil)
	if err != nil {
		return result, err
	}
	req.Header.Set("Authorization", newToken)

	resp, err := APIClient.Do(req)
	if err != nil {
		return result, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return result, fmt.Errorf("list users: HTTP %d: %s", resp.StatusCode, string(body))
	}

	err = json.NewDecoder(resp.Body).Decode(&result)
	return result, err
}
