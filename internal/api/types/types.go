package apitypes

import (
	"errors"
	"time"

	"github.com/gogrlx/grlx/v2/internal/cook"
	"github.com/gogrlx/grlx/v2/internal/pki"
	"github.com/gogrlx/grlx/v2/internal/rbac"
)

type (
	CmdCook struct {
		Async   bool            `json:"async"`
		Env     string          `json:"env"`
		Recipe  cook.RecipeName `json:"recipe"`
		Test    bool            `json:"test"`
		Timeout time.Duration   `json:"timeout"`

		Errors map[string]error `json:"errors"`
		JID    string           `json:"jid"`
	}
	CmdRun struct {
		Command string        `json:"command"`
		Args    []string      `json:"args"`
		Path    string        `json:"path"`
		CWD     string        `json:"cwd"`
		RunAs   string        `json:"runas"`
		Env     EnvVar        `json:"env"`
		Timeout time.Duration `json:"timeout"`

		// StreamTopic, when set, enables live output streaming over NATS.
		// Each chunk of stdout/stderr is published to this topic as JSON.
		StreamTopic string `json:"stream_topic,omitempty"`

		Stdout   string        `json:"stdout"`
		Stderr   string        `json:"stderr"`
		Duration time.Duration `json:"duration"`
		ErrCode  int           `json:"errcode"`

		Error error `json:"error"`
	}
	EnvVar map[string]string

	TargetedAction struct {
		Target []pki.KeyManager `json:"target"`
		Action interface{}      `json:"action"`
	}
	TargetedResults struct {
		Results map[string]interface{} `json:"results,omitempty"`
	}
	Inline struct {
		Success bool  `json:"success"`
		Error   error `json:"error"`
	}
	PingPong struct {
		Ping  bool  `json:"ping"`
		Pong  bool  `json:"pong"`
		Error error `json:"error"`
	}
	FilePath struct {
		Name string `json:"name"`
	}
)

// UserInfo represents a user's identity and role.
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

// ExplainResponse describes what the authenticated user can do.
type ExplainResponse struct {
	Pubkey   string               `json:"pubkey"`
	RoleName string               `json:"role"`
	IsAdmin  bool                 `json:"isAdmin"`
	Actions  []ActionExplain      `json:"actions"`
	Warnings []rbac.PolicyWarning `json:"warnings,omitempty"`
}

// ActionExplain describes a single permitted action.
type ActionExplain struct {
	Action rbac.Action `json:"action"`
	Scope  string      `json:"scope"`
}

var (
	ErrAPIRouteNotFound = errors.New("API Route not found")
	ErrInvalidUserInput = errors.New("invalid user input was received")
)
