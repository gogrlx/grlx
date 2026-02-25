package apitypes

import (
	"errors"
	"time"

	"github.com/gogrlx/grlx/v2/internal/cook"
	"github.com/gogrlx/grlx/v2/internal/pki"
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

var (
	ErrAPIRouteNotFound = errors.New("API Route not found")
	ErrInvalidUserInput = errors.New("invalid user input was received")
)
