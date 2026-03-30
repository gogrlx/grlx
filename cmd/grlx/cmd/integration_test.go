package cmd

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"

	"github.com/gogrlx/grlx/v2/internal/api/client"
	apitypes "github.com/gogrlx/grlx/v2/internal/api/types"
	"github.com/gogrlx/grlx/v2/internal/audit"
	"github.com/gogrlx/grlx/v2/internal/config"
	"github.com/gogrlx/grlx/v2/internal/jobs"
	"github.com/gogrlx/grlx/v2/internal/rbac"
)

// startTestNATSServer creates an embedded NATS server and returns a client
// connection. The server and connection are automatically cleaned up.
func startTestNATSServer(t *testing.T) (*server.Server, *nats.Conn) {
	t.Helper()
	opts := &server.Options{
		Host: "127.0.0.1",
		Port: -1,
	}
	ns, err := server.NewServer(opts)
	if err != nil {
		t.Fatalf("start test NATS server: %v", err)
	}
	go ns.Start()
	if !ns.ReadyForConnections(5 * time.Second) {
		t.Fatal("NATS server failed to become ready")
	}

	conn, err := nats.Connect(ns.ClientURL())
	if err != nil {
		ns.Shutdown()
		t.Fatalf("connect to test NATS: %v", err)
	}

	t.Cleanup(func() {
		conn.Close()
		ns.Shutdown()
	})

	return ns, conn
}

// natsRespond wraps a result in the standard nats response envelope.
func natsRespond(msg *nats.Msg, result interface{}) {
	resultBytes, _ := json.Marshal(result)
	resp := struct {
		Result json.RawMessage `json:"result"`
	}{Result: resultBytes}
	data, _ := json.Marshal(resp)
	msg.Respond(data)
}

// setupTestNATS sets up an embedded NATS server and configures client.NatsConn.
// Returns a cleanup function to restore the original connection.
func setupTestNATS(t *testing.T) (*nats.Conn, func()) {
	t.Helper()
	_, conn := startTestNATSServer(t)
	origConn := client.NatsConn
	client.NatsConn = conn
	return conn, func() {
		client.NatsConn = origConn
	}
}

// --- Sprouts commands ---

func TestSproutsListCommand_TextOutput(t *testing.T) {
	conn, cleanup := setupTestNATS(t)
	defer cleanup()

	// Mock sprouts.list response.
	conn.Subscribe("grlx.api.sprouts.list", func(msg *nats.Msg) {
		result := client.SproutListResponse{
			Sprouts: []client.SproutInfo{
				{ID: "web-1", KeyState: "accepted", Connected: true},
				{ID: "db-1", KeyState: "accepted", Connected: false},
				{ID: "pending-1", KeyState: "unaccepted", Connected: false},
			},
		}
		natsRespond(msg, result)
	})

	oldMode := outputMode
	defer func() { outputMode = oldMode }()
	outputMode = ""

	// Save and restore filter state.
	oldState := sproutsStateFilter
	oldOnline := sproutsOnlineOnly
	defer func() { sproutsStateFilter = oldState; sproutsOnlineOnly = oldOnline }()
	sproutsStateFilter = ""
	sproutsOnlineOnly = false

	out := captureStdout(t, func() {
		cmdSproutsList.Run(cmdSproutsList, nil)
	})

	if !strings.Contains(out, "web-1") {
		t.Error("expected web-1 in output")
	}
	if !strings.Contains(out, "db-1") {
		t.Error("expected db-1 in output")
	}
	if !strings.Contains(out, "pending-1") {
		t.Error("expected pending-1 in output")
	}
	if !strings.Contains(out, "online") {
		t.Error("expected 'online' status in output")
	}
	if !strings.Contains(out, "2 sprout(s) accepted") {
		t.Error("expected summary line")
	}
}

func TestSproutsListCommand_JSONOutput(t *testing.T) {
	conn, cleanup := setupTestNATS(t)
	defer cleanup()

	conn.Subscribe("grlx.api.sprouts.list", func(msg *nats.Msg) {
		result := client.SproutListResponse{
			Sprouts: []client.SproutInfo{
				{ID: "web-1", KeyState: "accepted", Connected: true},
			},
		}
		natsRespond(msg, result)
	})

	oldMode := outputMode
	defer func() { outputMode = oldMode }()
	outputMode = "json"

	oldState := sproutsStateFilter
	oldOnline := sproutsOnlineOnly
	defer func() { sproutsStateFilter = oldState; sproutsOnlineOnly = oldOnline }()
	sproutsStateFilter = ""
	sproutsOnlineOnly = false

	out := captureStdout(t, func() {
		cmdSproutsList.Run(cmdSproutsList, nil)
	})

	if !strings.Contains(out, `"web-1"`) {
		t.Error("expected web-1 in JSON output")
	}
	var parsed []client.SproutInfo
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &parsed); err != nil {
		t.Errorf("output is not valid JSON: %v", err)
	}
}

func TestSproutsListCommand_OnlineFilter(t *testing.T) {
	conn, cleanup := setupTestNATS(t)
	defer cleanup()

	conn.Subscribe("grlx.api.sprouts.list", func(msg *nats.Msg) {
		result := client.SproutListResponse{
			Sprouts: []client.SproutInfo{
				{ID: "web-1", KeyState: "accepted", Connected: true},
				{ID: "db-1", KeyState: "accepted", Connected: false},
			},
		}
		natsRespond(msg, result)
	})

	oldMode := outputMode
	oldState := sproutsStateFilter
	oldOnline := sproutsOnlineOnly
	defer func() {
		outputMode = oldMode
		sproutsStateFilter = oldState
		sproutsOnlineOnly = oldOnline
	}()
	outputMode = ""
	sproutsStateFilter = ""
	sproutsOnlineOnly = true

	out := captureStdout(t, func() {
		cmdSproutsList.Run(cmdSproutsList, nil)
	})

	if !strings.Contains(out, "web-1") {
		t.Error("expected web-1 in filtered output")
	}
	if strings.Contains(out, "db-1") {
		t.Error("db-1 should be filtered out (offline)")
	}
}

func TestSproutsListCommand_StateFilter(t *testing.T) {
	conn, cleanup := setupTestNATS(t)
	defer cleanup()

	conn.Subscribe("grlx.api.sprouts.list", func(msg *nats.Msg) {
		result := client.SproutListResponse{
			Sprouts: []client.SproutInfo{
				{ID: "web-1", KeyState: "accepted", Connected: true},
				{ID: "pending-1", KeyState: "unaccepted", Connected: false},
			},
		}
		natsRespond(msg, result)
	})

	oldMode := outputMode
	oldState := sproutsStateFilter
	oldOnline := sproutsOnlineOnly
	defer func() {
		outputMode = oldMode
		sproutsStateFilter = oldState
		sproutsOnlineOnly = oldOnline
	}()
	outputMode = ""
	sproutsStateFilter = "unaccepted"
	sproutsOnlineOnly = false

	out := captureStdout(t, func() {
		cmdSproutsList.Run(cmdSproutsList, nil)
	})

	if strings.Contains(out, "web-1") {
		t.Error("web-1 should be filtered out (accepted)")
	}
	if !strings.Contains(out, "pending-1") {
		t.Error("expected pending-1 in filtered output")
	}
}

func TestSproutsListCommand_Empty(t *testing.T) {
	conn, cleanup := setupTestNATS(t)
	defer cleanup()

	conn.Subscribe("grlx.api.sprouts.list", func(msg *nats.Msg) {
		result := client.SproutListResponse{Sprouts: []client.SproutInfo{}}
		natsRespond(msg, result)
	})

	oldMode := outputMode
	oldState := sproutsStateFilter
	oldOnline := sproutsOnlineOnly
	defer func() {
		outputMode = oldMode
		sproutsStateFilter = oldState
		sproutsOnlineOnly = oldOnline
	}()
	outputMode = ""
	sproutsStateFilter = ""
	sproutsOnlineOnly = false

	out := captureStdout(t, func() {
		cmdSproutsList.Run(cmdSproutsList, nil)
	})

	if !strings.Contains(out, "No sprouts found") {
		t.Error("expected 'No sprouts found' message")
	}
}

func TestSproutsShowCommand_Text(t *testing.T) {
	conn, cleanup := setupTestNATS(t)
	defer cleanup()

	conn.Subscribe("grlx.api.sprouts.get", func(msg *nats.Msg) {
		result := client.SproutInfo{
			ID:        "web-1",
			KeyState:  "accepted",
			Connected: true,
			NKey:      "NKEY_XYZ123",
		}
		natsRespond(msg, result)
	})
	conn.Subscribe("grlx.api.props.getall", func(msg *nats.Msg) {
		result := map[string]interface{}{
			"os":       "linux",
			"hostname": "web-1.example.com",
		}
		natsRespond(msg, result)
	})

	oldMode := outputMode
	defer func() { outputMode = oldMode }()
	outputMode = ""

	out := captureStdout(t, func() {
		cmdSproutsShow.Run(cmdSproutsShow, []string{"web-1"})
	})

	if !strings.Contains(out, "web-1") {
		t.Error("expected sprout ID in output")
	}
	if !strings.Contains(out, "accepted") {
		t.Error("expected key state in output")
	}
	if !strings.Contains(out, "NKEY_XYZ123") {
		t.Error("expected NKey in output")
	}
	if !strings.Contains(out, "Properties") {
		t.Error("expected Properties section")
	}
	if !strings.Contains(out, "hostname") {
		t.Error("expected hostname prop")
	}
}

func TestSproutsShowCommand_JSON(t *testing.T) {
	conn, cleanup := setupTestNATS(t)
	defer cleanup()

	conn.Subscribe("grlx.api.sprouts.get", func(msg *nats.Msg) {
		result := client.SproutInfo{
			ID:        "web-1",
			KeyState:  "accepted",
			Connected: true,
		}
		natsRespond(msg, result)
	})
	conn.Subscribe("grlx.api.props.getall", func(msg *nats.Msg) {
		result := map[string]interface{}{"os": "linux"}
		natsRespond(msg, result)
	})

	oldMode := outputMode
	defer func() { outputMode = oldMode }()
	outputMode = "json"

	out := captureStdout(t, func() {
		cmdSproutsShow.Run(cmdSproutsShow, []string{"web-1"})
	})

	if !strings.Contains(out, `"web-1"`) {
		t.Error("expected sprout ID in JSON output")
	}
}

// --- Cohorts commands ---

func TestCohortsListCommand_Text(t *testing.T) {
	conn, cleanup := setupTestNATS(t)
	defer cleanup()

	conn.Subscribe("grlx.api.cohorts.list", func(msg *nats.Msg) {
		result := struct {
			Cohorts []struct {
				Name string `json:"name"`
				Type string `json:"type"`
			} `json:"cohorts"`
		}{
			Cohorts: []struct {
				Name string `json:"name"`
				Type string `json:"type"`
			}{
				{Name: "web-servers", Type: "static"},
				{Name: "production", Type: "dynamic"},
			},
		}
		natsRespond(msg, result)
	})

	oldMode := outputMode
	defer func() { outputMode = oldMode }()
	outputMode = ""

	out := captureStdout(t, func() {
		cmdCohortsList.Run(cmdCohortsList, nil)
	})

	if !strings.Contains(out, "web-servers") {
		t.Error("expected web-servers in output")
	}
	if !strings.Contains(out, "static") {
		t.Error("expected 'static' type")
	}
	if !strings.Contains(out, "production") {
		t.Error("expected production in output")
	}
}

func TestCohortsListCommand_Empty(t *testing.T) {
	conn, cleanup := setupTestNATS(t)
	defer cleanup()

	conn.Subscribe("grlx.api.cohorts.list", func(msg *nats.Msg) {
		result := struct {
			Cohorts []struct {
				Name string `json:"name"`
				Type string `json:"type"`
			} `json:"cohorts"`
		}{}
		natsRespond(msg, result)
	})

	oldMode := outputMode
	defer func() { outputMode = oldMode }()
	outputMode = ""

	out := captureStdout(t, func() {
		cmdCohortsList.Run(cmdCohortsList, nil)
	})

	if !strings.Contains(out, "No cohorts configured") {
		t.Error("expected 'No cohorts configured' message")
	}
}

func TestCohortsResolveCommand_Text(t *testing.T) {
	conn, cleanup := setupTestNATS(t)
	defer cleanup()

	conn.Subscribe("grlx.api.cohorts.resolve", func(msg *nats.Msg) {
		result := struct {
			Sprouts []string `json:"sprouts"`
		}{
			Sprouts: []string{"web-1", "web-2", "web-3"},
		}
		natsRespond(msg, result)
	})

	oldMode := outputMode
	defer func() { outputMode = oldMode }()
	outputMode = ""

	out := captureStdout(t, func() {
		cmdCohortsResolve.Run(cmdCohortsResolve, []string{"web-servers"})
	})

	if !strings.Contains(out, "web-servers") {
		t.Error("expected cohort name in output")
	}
	if !strings.Contains(out, "3 sprout(s)") {
		t.Error("expected sprout count in output")
	}
	if !strings.Contains(out, "web-1") {
		t.Error("expected web-1 in output")
	}
}

func TestCohortsResolveCommand_JSON(t *testing.T) {
	conn, cleanup := setupTestNATS(t)
	defer cleanup()

	conn.Subscribe("grlx.api.cohorts.resolve", func(msg *nats.Msg) {
		result := struct {
			Sprouts []string `json:"sprouts"`
		}{
			Sprouts: []string{"web-1", "web-2"},
		}
		natsRespond(msg, result)
	})

	oldMode := outputMode
	defer func() { outputMode = oldMode }()
	outputMode = "json"

	out := captureStdout(t, func() {
		cmdCohortsResolve.Run(cmdCohortsResolve, []string{"web-servers"})
	})

	if !strings.Contains(out, `"web-1"`) {
		t.Error("expected web-1 in JSON output")
	}
}

// --- Version command ---

func TestVersionCommand_Text(t *testing.T) {
	conn, cleanup := setupTestNATS(t)
	defer cleanup()

	conn.Subscribe("grlx.api.version", func(msg *nats.Msg) {
		result := config.Version{
			Tag:       "v2.1.0",
			GitCommit: "abc123",
			Arch:      "linux/amd64",
			Compiler:  "go1.23.0",
		}
		natsRespond(msg, result)
	})

	oldMode := outputMode
	oldBI := BuildInfo
	defer func() { outputMode = oldMode; BuildInfo = oldBI }()
	outputMode = ""
	BuildInfo = config.Version{
		Tag:       "v2.1.0-test",
		GitCommit: "test123",
		Arch:      "linux/amd64",
		Compiler:  "go1.23.0",
	}

	out := captureStdout(t, func() {
		versionCmd.Run(versionCmd, nil)
	})

	if !strings.Contains(out, "v2.1.0-test") {
		t.Error("expected CLI version tag in output")
	}
	if !strings.Contains(out, "test123") {
		t.Error("expected CLI commit in output")
	}
	if !strings.Contains(out, "v2.1.0") {
		t.Error("expected farmer version in output")
	}
}

func TestVersionCommand_JSON(t *testing.T) {
	conn, cleanup := setupTestNATS(t)
	defer cleanup()

	conn.Subscribe("grlx.api.version", func(msg *nats.Msg) {
		result := config.Version{Tag: "v2.1.0"}
		natsRespond(msg, result)
	})

	oldMode := outputMode
	oldBI := BuildInfo
	defer func() { outputMode = oldMode; BuildInfo = oldBI }()
	outputMode = "json"
	BuildInfo = config.Version{Tag: "v2.1.0-test"}

	out := captureStdout(t, func() {
		versionCmd.Run(versionCmd, nil)
	})

	var parsed config.CombinedVersion
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &parsed); err != nil {
		t.Errorf("output is not valid JSON: %v", err)
	}
	if parsed.CLI.Tag != "v2.1.0-test" {
		t.Errorf("expected CLI tag v2.1.0-test, got %s", parsed.CLI.Tag)
	}
}

// --- Recipes commands ---

func TestRecipesListCommand_Text(t *testing.T) {
	conn, cleanup := setupTestNATS(t)
	defer cleanup()

	conn.Subscribe("grlx.api.recipes.list", func(msg *nats.Msg) {
		result := struct {
			Recipes []RecipeInfo `json:"recipes"`
		}{
			Recipes: []RecipeInfo{
				{Name: "base.packages", Path: "/srv/recipes/base/packages.grlx", Size: 1024},
				{Name: "webserver.nginx", Path: "/srv/recipes/webserver/nginx.grlx", Size: 2048},
			},
		}
		natsRespond(msg, result)
	})

	oldMode := outputMode
	defer func() { outputMode = oldMode }()
	outputMode = ""

	out := captureStdout(t, func() {
		cmdRecipesList.Run(cmdRecipesList, nil)
	})

	if !strings.Contains(out, "base.packages") {
		t.Error("expected base.packages in output")
	}
	if !strings.Contains(out, "webserver.nginx") {
		t.Error("expected webserver.nginx in output")
	}
	if !strings.Contains(out, "2 recipe(s)") {
		t.Error("expected recipe count")
	}
}

func TestRecipesListCommand_Empty(t *testing.T) {
	conn, cleanup := setupTestNATS(t)
	defer cleanup()

	conn.Subscribe("grlx.api.recipes.list", func(msg *nats.Msg) {
		result := struct {
			Recipes []RecipeInfo `json:"recipes"`
		}{}
		natsRespond(msg, result)
	})

	oldMode := outputMode
	defer func() { outputMode = oldMode }()
	outputMode = ""

	out := captureStdout(t, func() {
		cmdRecipesList.Run(cmdRecipesList, nil)
	})

	if !strings.Contains(out, "No recipes found") {
		t.Error("expected 'No recipes found' message")
	}
}

func TestRecipesShowCommand_Text(t *testing.T) {
	conn, cleanup := setupTestNATS(t)
	defer cleanup()

	conn.Subscribe("grlx.api.recipes.get", func(msg *nats.Msg) {
		result := RecipeContent{
			Name:    "base.packages",
			Path:    "/srv/recipes/base/packages.grlx",
			Content: "pkg.installed:\n  - name: nginx",
			Size:    42,
		}
		natsRespond(msg, result)
	})

	oldMode := outputMode
	defer func() { outputMode = oldMode }()
	outputMode = ""

	out := captureStdout(t, func() {
		cmdRecipesShow.Run(cmdRecipesShow, []string{"base.packages"})
	})

	if !strings.Contains(out, "base.packages") {
		t.Error("expected recipe name in output")
	}
	if !strings.Contains(out, "pkg.installed") {
		t.Error("expected recipe content in output")
	}
}

// --- Cohorts show command ---

func TestCohortsShowCommand_Static(t *testing.T) {
	conn, cleanup := setupTestNATS(t)
	defer cleanup()

	conn.Subscribe("grlx.api.cohorts.get", func(msg *nats.Msg) {
		result := client.CohortDetail{
			Name:     "web-servers",
			Type:     "static",
			Members:  []string{"web-1", "web-2"},
			Resolved: []string{"web-1", "web-2"},
			Count:    2,
		}
		natsRespond(msg, result)
	})

	oldMode := outputMode
	defer func() { outputMode = oldMode }()
	outputMode = ""

	out := captureStdout(t, func() {
		cmdCohortsShow.Run(cmdCohortsShow, []string{"web-servers"})
	})

	if !strings.Contains(out, "web-servers") {
		t.Error("expected cohort name in output")
	}
	if !strings.Contains(out, "static") {
		t.Error("expected type in output")
	}
	if !strings.Contains(out, "web-1") {
		t.Error("expected member web-1")
	}
}

// --- Roles command ---

func TestRolesCommand_Text(t *testing.T) {
	conn, cleanup := setupTestNATS(t)
	defer cleanup()

	conn.Subscribe("grlx.api.auth.users", func(msg *nats.Msg) {
		result := apitypes.UsersListResponse{
			Users: map[string]string{"PUBKEY1": "viewer"},
			Roles: []apitypes.RoleInfo{
				{
					Name:  "viewer",
					Rules: []rbac.Rule{{Action: "view", Scope: "*"}},
				},
				{
					Name:  "operator",
					Rules: []rbac.Rule{{Action: "cook", Scope: "web-*"}, {Action: "view", Scope: "*"}},
				},
			},
		}
		natsRespond(msg, result)
	})

	oldMode := outputMode
	defer func() { outputMode = oldMode }()
	outputMode = ""

	out := captureStdout(t, func() {
		rolesCmd.Run(rolesCmd, nil)
	})

	if !strings.Contains(out, "viewer") {
		t.Error("expected viewer role")
	}
	if !strings.Contains(out, "built-in") {
		t.Error("expected (built-in) suffix")
	}
	if !strings.Contains(out, "operator") {
		t.Error("expected operator role")
	}
}

func TestRolesCommand_JSON(t *testing.T) {
	conn, cleanup := setupTestNATS(t)
	defer cleanup()

	conn.Subscribe("grlx.api.auth.users", func(msg *nats.Msg) {
		result := apitypes.UsersListResponse{
			Roles: []apitypes.RoleInfo{
				{Name: "viewer", Rules: []rbac.Rule{{Action: "view", Scope: "*"}}},
			},
		}
		natsRespond(msg, result)
	})

	oldMode := outputMode
	defer func() { outputMode = oldMode }()
	outputMode = "json"

	out := captureStdout(t, func() {
		rolesCmd.Run(rolesCmd, nil)
	})

	if !strings.Contains(out, "viewer") {
		t.Error("expected viewer in JSON output")
	}
	if !strings.Contains(out, `"builtin":true`) {
		t.Error("expected builtin:true in JSON output")
	}
}

func TestRolesCommand_Empty(t *testing.T) {
	conn, cleanup := setupTestNATS(t)
	defer cleanup()

	conn.Subscribe("grlx.api.auth.users", func(msg *nats.Msg) {
		result := apitypes.UsersListResponse{Roles: []apitypes.RoleInfo{}}
		natsRespond(msg, result)
	})

	oldMode := outputMode
	defer func() { outputMode = oldMode }()
	outputMode = ""

	out := captureStdout(t, func() {
		rolesCmd.Run(rolesCmd, nil)
	})

	if !strings.Contains(out, "No roles configured") {
		t.Error("expected 'No roles configured' message")
	}
}

// --- Users commands ---

func TestUsersListCommand_Text(t *testing.T) {
	conn, cleanup := setupTestNATS(t)
	defer cleanup()

	conn.Subscribe("grlx.api.auth.users", func(msg *nats.Msg) {
		result := apitypes.UsersListResponse{
			Users: map[string]string{
				"PUBKEY_ADMIN":  "admin",
				"PUBKEY_VIEWER": "viewer",
			},
		}
		natsRespond(msg, result)
	})

	oldMode := outputMode
	defer func() { outputMode = oldMode }()
	outputMode = ""

	out := captureStdout(t, func() {
		usersListCmd.Run(usersListCmd, nil)
	})

	if !strings.Contains(out, "PUBKEY_ADMIN") {
		t.Error("expected admin pubkey")
	}
	if !strings.Contains(out, "viewer") {
		t.Error("expected viewer role")
	}
}

func TestUsersListCommand_Empty(t *testing.T) {
	conn, cleanup := setupTestNATS(t)
	defer cleanup()

	conn.Subscribe("grlx.api.auth.users", func(msg *nats.Msg) {
		result := apitypes.UsersListResponse{Users: map[string]string{}}
		natsRespond(msg, result)
	})

	oldMode := outputMode
	defer func() { outputMode = oldMode }()
	outputMode = ""

	out := captureStdout(t, func() {
		usersListCmd.Run(usersListCmd, nil)
	})

	if !strings.Contains(out, "No users configured") {
		t.Error("expected 'No users configured' message")
	}
}

// --- Cohorts refresh ---

func TestCohortsRefreshCommand_Text(t *testing.T) {
	conn, cleanup := setupTestNATS(t)
	defer cleanup()

	conn.Subscribe("grlx.api.cohorts.refresh", func(msg *nats.Msg) {
		result := struct {
			Refreshed []struct {
				Name          string   `json:"name"`
				Members       []string `json:"members"`
				LastRefreshed string   `json:"lastRefreshed"`
			} `json:"refreshed"`
		}{
			Refreshed: []struct {
				Name          string   `json:"name"`
				Members       []string `json:"members"`
				LastRefreshed string   `json:"lastRefreshed"`
			}{
				{Name: "web-servers", Members: []string{"web-1", "web-2"}, LastRefreshed: "2026-03-28T03:00:00Z"},
			},
		}
		natsRespond(msg, result)
	})

	oldMode := outputMode
	defer func() { outputMode = oldMode }()
	outputMode = ""

	out := captureStdout(t, func() {
		cmdCohortsRefresh.Run(cmdCohortsRefresh, nil)
	})

	if !strings.Contains(out, "web-servers") {
		t.Error("expected cohort name in refresh output")
	}
	if !strings.Contains(out, "2 sprout(s)") {
		t.Error("expected sprout count")
	}
}

func TestCohortsRefreshCommand_Empty(t *testing.T) {
	conn, cleanup := setupTestNATS(t)
	defer cleanup()

	conn.Subscribe("grlx.api.cohorts.refresh", func(msg *nats.Msg) {
		result := struct {
			Refreshed []struct{} `json:"refreshed"`
		}{}
		natsRespond(msg, result)
	})

	oldMode := outputMode
	defer func() { outputMode = oldMode }()
	outputMode = ""

	out := captureStdout(t, func() {
		cmdCohortsRefresh.Run(cmdCohortsRefresh, nil)
	})

	if !strings.Contains(out, "No cohorts to refresh") {
		t.Error("expected 'No cohorts to refresh' message")
	}
}

// --- Targeting with cohort resolution ---

// --- Auth whoami command ---

func TestAuthWhoAmICommand_Text(t *testing.T) {
	conn, cleanup := setupTestNATS(t)
	defer cleanup()

	conn.Subscribe("grlx.api.auth.whoami", func(msg *nats.Msg) {
		result := apitypes.UserInfo{
			Pubkey:   "ABCDEF123456",
			RoleName: "operator",
		}
		natsRespond(msg, result)
	})

	oldMode := outputMode
	defer func() { outputMode = oldMode }()
	outputMode = ""

	out := captureStdout(t, func() {
		authWhoAmICmd.Run(authWhoAmICmd, nil)
	})

	if !strings.Contains(out, "ABCDEF123456") {
		t.Error("expected pubkey in output")
	}
	if !strings.Contains(out, "operator") {
		t.Error("expected role name in output")
	}
}

func TestAuthWhoAmICommand_JSON(t *testing.T) {
	conn, cleanup := setupTestNATS(t)
	defer cleanup()

	conn.Subscribe("grlx.api.auth.whoami", func(msg *nats.Msg) {
		result := apitypes.UserInfo{
			Pubkey:   "KEY999",
			RoleName: "admin",
		}
		natsRespond(msg, result)
	})

	oldMode := outputMode
	defer func() { outputMode = oldMode }()
	outputMode = "json"

	out := captureStdout(t, func() {
		authWhoAmICmd.Run(authWhoAmICmd, nil)
	})

	if !strings.Contains(out, `"pubkey"`) {
		t.Error("expected JSON pubkey field")
	}
	if !strings.Contains(out, "KEY999") {
		t.Error("expected pubkey value in JSON output")
	}
}

// --- Auth explain command ---

func TestAuthExplainCommand_Text(t *testing.T) {
	conn, cleanup := setupTestNATS(t)
	defer cleanup()

	conn.Subscribe("grlx.api.auth.explain", func(msg *nats.Msg) {
		result := apitypes.ExplainResponse{
			Pubkey:   "USERKEY",
			RoleName: "operator",
			IsAdmin:  false,
			Actions: []apitypes.ActionExplain{
				{Action: "cook", Scope: "web-*"},
				{Action: "view", Scope: "*"},
			},
		}
		natsRespond(msg, result)
	})

	oldMode := outputMode
	defer func() { outputMode = oldMode }()
	outputMode = ""

	out := captureStdout(t, func() {
		authExplainCmd.Run(authExplainCmd, nil)
	})

	if !strings.Contains(out, "USERKEY") {
		t.Error("expected pubkey in output")
	}
	if !strings.Contains(out, "operator") {
		t.Error("expected role in output")
	}
	if !strings.Contains(out, "cook") {
		t.Error("expected cook action in output")
	}
}

func TestAuthExplainCommand_Admin(t *testing.T) {
	conn, cleanup := setupTestNATS(t)
	defer cleanup()

	conn.Subscribe("grlx.api.auth.explain", func(msg *nats.Msg) {
		result := apitypes.ExplainResponse{
			Pubkey:   "ADMINKEY",
			RoleName: "admin",
			IsAdmin:  true,
		}
		natsRespond(msg, result)
	})

	oldMode := outputMode
	defer func() { outputMode = oldMode }()
	outputMode = ""

	out := captureStdout(t, func() {
		authExplainCmd.Run(authExplainCmd, nil)
	})

	if !strings.Contains(out, "Admin:") {
		t.Error("expected Admin line in output")
	}
}

func TestAuthExplainCommand_WithWarnings(t *testing.T) {
	conn, cleanup := setupTestNATS(t)
	defer cleanup()

	conn.Subscribe("grlx.api.auth.explain", func(msg *nats.Msg) {
		result := apitypes.ExplainResponse{
			Pubkey:   "WARNKEY",
			RoleName: "viewer",
			Warnings: []rbac.PolicyWarning{
				{Kind: "orphan_role_ref", Message: "role 'ghost' is referenced but not defined"},
			},
		}
		natsRespond(msg, result)
	})

	oldMode := outputMode
	defer func() { outputMode = oldMode }()
	outputMode = ""

	out := captureStdout(t, func() {
		authExplainCmd.Run(authExplainCmd, nil)
	})

	if !strings.Contains(out, "Warnings:") {
		t.Error("expected Warnings section")
	}
	if !strings.Contains(out, "orphan_role_ref") {
		t.Error("expected warning kind in output")
	}
}

func TestAuthExplainCommand_JSON(t *testing.T) {
	conn, cleanup := setupTestNATS(t)
	defer cleanup()

	conn.Subscribe("grlx.api.auth.explain", func(msg *nats.Msg) {
		result := apitypes.ExplainResponse{
			Pubkey:   "JSON-KEY",
			RoleName: "admin",
			IsAdmin:  true,
		}
		natsRespond(msg, result)
	})

	oldMode := outputMode
	defer func() { outputMode = oldMode }()
	outputMode = "json"

	out := captureStdout(t, func() {
		authExplainCmd.Run(authExplainCmd, nil)
	})

	if !strings.Contains(out, "JSON-KEY") {
		t.Error("expected pubkey in JSON output")
	}
	if !strings.Contains(out, `"isAdmin":true`) {
		t.Error("expected isAdmin field in JSON output")
	}
}

// --- Auth users list (via auth subcommand) ---

func TestAuthUsersCommand_Text(t *testing.T) {
	conn, cleanup := setupTestNATS(t)
	defer cleanup()

	conn.Subscribe("grlx.api.auth.users", func(msg *nats.Msg) {
		result := apitypes.UsersListResponse{
			Users: map[string]string{
				"PUBKEY-A": "admin",
				"PUBKEY-B": "operator",
			},
			Roles: []apitypes.RoleInfo{
				{Name: "admin", Rules: []rbac.Rule{{Action: "admin", Scope: "*"}}},
			},
		}
		natsRespond(msg, result)
	})

	oldMode := outputMode
	defer func() { outputMode = oldMode }()
	outputMode = ""

	out := captureStdout(t, func() {
		authUsersCmd.Run(authUsersCmd, nil)
	})

	if !strings.Contains(out, "Users:") {
		t.Error("expected Users header")
	}
	if !strings.Contains(out, "admin") {
		t.Error("expected admin role in output")
	}
}

// --- Auth roles list (via auth subcommand) ---

func TestAuthRolesCommand_Text(t *testing.T) {
	conn, cleanup := setupTestNATS(t)
	defer cleanup()

	conn.Subscribe("grlx.api.auth.users", func(msg *nats.Msg) {
		result := apitypes.UsersListResponse{
			Roles: []apitypes.RoleInfo{
				{Name: "operator", Rules: []rbac.Rule{
					{Action: "cook", Scope: "web-*"},
					{Action: "view", Scope: "*"},
				}},
			},
		}
		natsRespond(msg, result)
	})

	oldMode := outputMode
	defer func() { outputMode = oldMode }()
	outputMode = ""

	out := captureStdout(t, func() {
		authRolesCmd.Run(authRolesCmd, nil)
	})

	if !strings.Contains(out, "operator") {
		t.Error("expected operator role in output")
	}
	if !strings.Contains(out, "cook") {
		t.Error("expected cook action in output")
	}
}

func TestAuthRolesCommand_JSON(t *testing.T) {
	conn, cleanup := setupTestNATS(t)
	defer cleanup()

	conn.Subscribe("grlx.api.auth.users", func(msg *nats.Msg) {
		result := apitypes.UsersListResponse{
			Roles: []apitypes.RoleInfo{
				{Name: "viewer", Rules: []rbac.Rule{{Action: "view", Scope: "*"}}},
			},
		}
		natsRespond(msg, result)
	})

	oldMode := outputMode
	defer func() { outputMode = oldMode }()
	outputMode = "json"

	out := captureStdout(t, func() {
		authRolesCmd.Run(authRolesCmd, nil)
	})

	if !strings.Contains(out, "viewer") {
		t.Error("expected viewer role in JSON output")
	}
}

// --- Audit dates command ---

func TestAuditDatesCommand_Text(t *testing.T) {
	conn, cleanup := setupTestNATS(t)
	defer cleanup()

	conn.Subscribe("grlx.api.audit.dates", func(msg *nats.Msg) {
		result := []audit.DateSummary{
			{Date: "2026-03-28", EntryCount: 42, SizeBytes: 8192},
			{Date: "2026-03-27", EntryCount: 15, SizeBytes: 3072},
		}
		natsRespond(msg, result)
	})

	oldMode := outputMode
	defer func() { outputMode = oldMode }()
	outputMode = ""

	out := captureStdout(t, func() {
		cmdAuditDates.Run(cmdAuditDates, nil)
	})

	if !strings.Contains(out, "2026-03-28") {
		t.Error("expected date in output")
	}
	if !strings.Contains(out, "42") {
		t.Error("expected entry count in output")
	}
}

func TestAuditDatesCommand_Empty(t *testing.T) {
	conn, cleanup := setupTestNATS(t)
	defer cleanup()

	conn.Subscribe("grlx.api.audit.dates", func(msg *nats.Msg) {
		natsRespond(msg, []audit.DateSummary{})
	})

	oldMode := outputMode
	defer func() { outputMode = oldMode }()
	outputMode = ""

	out := captureStdout(t, func() {
		cmdAuditDates.Run(cmdAuditDates, nil)
	})

	if !strings.Contains(out, "No audit logs") {
		t.Error("expected 'No audit logs' message")
	}
}

func TestAuditDatesCommand_JSON(t *testing.T) {
	conn, cleanup := setupTestNATS(t)
	defer cleanup()

	conn.Subscribe("grlx.api.audit.dates", func(msg *nats.Msg) {
		result := []audit.DateSummary{
			{Date: "2026-03-28", EntryCount: 10, SizeBytes: 2048},
		}
		natsRespond(msg, result)
	})

	oldMode := outputMode
	defer func() { outputMode = oldMode }()
	outputMode = "json"

	out := captureStdout(t, func() {
		cmdAuditDates.Run(cmdAuditDates, nil)
	})

	if !strings.Contains(out, "2026-03-28") {
		t.Error("expected date in JSON output")
	}
}

// --- Audit list command ---

func TestAuditListCommand_Text(t *testing.T) {
	conn, cleanup := setupTestNATS(t)
	defer cleanup()

	conn.Subscribe("grlx.api.audit.query", func(msg *nats.Msg) {
		result := audit.QueryResult{
			Date:  "2026-03-28",
			Total: 2,
			Entries: []audit.Entry{
				{
					Timestamp: time.Date(2026, 3, 28, 14, 30, 0, 0, time.UTC),
					Action:    "cook",
					RoleName:  "operator",
					Targets:   []string{"web-1", "web-2"},
					Success:   true,
				},
				{
					Timestamp: time.Date(2026, 3, 28, 14, 35, 0, 0, time.UTC),
					Action:    "pki.accept",
					RoleName:  "admin",
					Success:   false,
					Error:     "sprout not found",
				},
			},
		}
		natsRespond(msg, result)
	})

	oldMode := outputMode
	defer func() { outputMode = oldMode }()
	outputMode = ""

	out := captureStdout(t, func() {
		cmdAuditList.Run(cmdAuditList, nil)
	})

	if !strings.Contains(out, "cook") {
		t.Error("expected cook action in output")
	}
	if !strings.Contains(out, "pki.accept") {
		t.Error("expected pki.accept action in output")
	}
	if !strings.Contains(out, "web-1") {
		t.Error("expected targets in output")
	}
}

func TestAuditListCommand_Empty(t *testing.T) {
	conn, cleanup := setupTestNATS(t)
	defer cleanup()

	conn.Subscribe("grlx.api.audit.query", func(msg *nats.Msg) {
		result := audit.QueryResult{
			Date:    "2026-03-28",
			Total:   0,
			Entries: []audit.Entry{},
		}
		natsRespond(msg, result)
	})

	oldMode := outputMode
	defer func() { outputMode = oldMode }()
	outputMode = ""

	out := captureStdout(t, func() {
		cmdAuditList.Run(cmdAuditList, nil)
	})

	if !strings.Contains(out, "No audit entries") {
		t.Error("expected 'No audit entries' message")
	}
}

func TestAuditListCommand_JSON(t *testing.T) {
	conn, cleanup := setupTestNATS(t)
	defer cleanup()

	conn.Subscribe("grlx.api.audit.query", func(msg *nats.Msg) {
		result := audit.QueryResult{
			Date:  "2026-03-28",
			Total: 1,
			Entries: []audit.Entry{
				{
					Timestamp: time.Date(2026, 3, 28, 14, 30, 0, 0, time.UTC),
					Action:    "cook",
					Success:   true,
				},
			},
		}
		natsRespond(msg, result)
	})

	oldMode := outputMode
	defer func() { outputMode = oldMode }()
	outputMode = "json"

	out := captureStdout(t, func() {
		cmdAuditList.Run(cmdAuditList, nil)
	})

	if !strings.Contains(out, "cook") {
		t.Error("expected cook action in JSON output")
	}
}

// --- Jobs list command (NATS path) ---

func TestJobsListCommand_Text(t *testing.T) {
	conn, cleanup := setupTestNATS(t)
	defer cleanup()

	conn.Subscribe("grlx.api.jobs.list", func(msg *nats.Msg) {
		result := []jobs.JobSummary{
			{
				JID:       "job-abc",
				SproutID:  "web-1",
				Status:    jobs.JobSucceeded,
				Succeeded: 5,
				StartedAt: time.Date(2026, 3, 28, 3, 0, 0, 0, time.UTC),
			},
		}
		natsRespond(msg, result)
	})

	oldMode, oldLocal := outputMode, jobsLocal
	defer func() { outputMode = oldMode; jobsLocal = oldLocal }()
	outputMode = ""
	jobsLocal = false

	out := captureStdout(t, func() {
		cmdJobsList.Run(cmdJobsList, nil)
	})

	if !strings.Contains(out, "job-abc") {
		t.Error("expected job JID in output")
	}
	if !strings.Contains(out, "web-1") {
		t.Error("expected sprout ID in output")
	}
}

func TestJobsListCommand_JSON(t *testing.T) {
	conn, cleanup := setupTestNATS(t)
	defer cleanup()

	conn.Subscribe("grlx.api.jobs.list", func(msg *nats.Msg) {
		result := []jobs.JobSummary{
			{JID: "job-xyz", SproutID: "db-1", Status: jobs.JobFailed},
		}
		natsRespond(msg, result)
	})

	oldMode, oldLocal := outputMode, jobsLocal
	defer func() { outputMode = oldMode; jobsLocal = oldLocal }()
	outputMode = "json"
	jobsLocal = false

	out := captureStdout(t, func() {
		cmdJobsList.Run(cmdJobsList, nil)
	})

	if !strings.Contains(out, "job-xyz") {
		t.Error("expected JID in JSON output")
	}
}

func TestJobsListCommand_Empty(t *testing.T) {
	conn, cleanup := setupTestNATS(t)
	defer cleanup()

	conn.Subscribe("grlx.api.jobs.list", func(msg *nats.Msg) {
		natsRespond(msg, []jobs.JobSummary{})
	})

	oldMode, oldLocal := outputMode, jobsLocal
	defer func() { outputMode = oldMode; jobsLocal = oldLocal }()
	outputMode = ""
	jobsLocal = false

	out := captureStdout(t, func() {
		cmdJobsList.Run(cmdJobsList, nil)
	})

	if !strings.Contains(out, "No jobs found") {
		t.Error("expected 'No jobs found' message")
	}
}

func TestJobsListCommand_BySprout(t *testing.T) {
	conn, cleanup := setupTestNATS(t)
	defer cleanup()

	conn.Subscribe("grlx.api.jobs.forsprout", func(msg *nats.Msg) {
		result := []jobs.JobSummary{
			{JID: "sprout-job-1", SproutID: "web-1", Status: jobs.JobSucceeded},
		}
		natsRespond(msg, result)
	})

	oldMode, oldLocal := outputMode, jobsLocal
	defer func() { outputMode = oldMode; jobsLocal = oldLocal }()
	outputMode = ""
	jobsLocal = false

	out := captureStdout(t, func() {
		cmdJobsList.Run(cmdJobsList, []string{"web-1"})
	})

	if !strings.Contains(out, "sprout-job-1") {
		t.Error("expected job JID in output")
	}
}

// --- Jobs show command (NATS path) ---

func TestJobsShowCommand_Text(t *testing.T) {
	conn, cleanup := setupTestNATS(t)
	defer cleanup()

	conn.Subscribe("grlx.api.jobs.get", func(msg *nats.Msg) {
		result := jobs.JobSummary{
			JID:       "show-job-1",
			SproutID:  "app-1",
			Status:    jobs.JobSucceeded,
			Total:     2,
			Succeeded: 2,
			StartedAt: time.Date(2026, 3, 28, 3, 0, 0, 0, time.UTC),
			Duration:  10 * time.Second,
		}
		natsRespond(msg, result)
	})

	oldMode, oldLocal := outputMode, jobsLocal
	defer func() { outputMode = oldMode; jobsLocal = oldLocal }()
	outputMode = ""
	jobsLocal = false

	out := captureStdout(t, func() {
		cmdJobsShow.Run(cmdJobsShow, []string{"show-job-1"})
	})

	if !strings.Contains(out, "show-job-1") {
		t.Error("expected JID in detail output")
	}
	if !strings.Contains(out, "app-1") {
		t.Error("expected sprout ID in detail output")
	}
}

func TestJobsShowCommand_JSON(t *testing.T) {
	conn, cleanup := setupTestNATS(t)
	defer cleanup()

	conn.Subscribe("grlx.api.jobs.get", func(msg *nats.Msg) {
		result := jobs.JobSummary{
			JID:      "json-job-1",
			SproutID: "db-1",
			Status:   jobs.JobFailed,
		}
		natsRespond(msg, result)
	})

	oldMode, oldLocal := outputMode, jobsLocal
	defer func() { outputMode = oldMode; jobsLocal = oldLocal }()
	outputMode = "json"
	jobsLocal = false

	out := captureStdout(t, func() {
		cmdJobsShow.Run(cmdJobsShow, []string{"json-job-1"})
	})

	if !strings.Contains(out, "json-job-1") {
		t.Error("expected JID in JSON output")
	}
}

// --- Jobs cancel command ---

func TestJobsCancelCommand(t *testing.T) {
	conn, cleanup := setupTestNATS(t)
	defer cleanup()

	conn.Subscribe("grlx.api.jobs.cancel", func(msg *nats.Msg) {
		natsRespond(msg, map[string]bool{"ok": true})
	})

	out := captureStdout(t, func() {
		cmdJobsCancel.Run(cmdJobsCancel, []string{"cancel-jid-1"})
	})

	if !strings.Contains(out, "cancel-jid-1") {
		t.Error("expected JID in cancel confirmation")
	}
	if !strings.Contains(out, "Cancel request sent") {
		t.Error("expected cancel confirmation message")
	}
}

// --- Users list command ---

func TestUsersListCommand_WithUsers(t *testing.T) {
	conn, cleanup := setupTestNATS(t)
	defer cleanup()

	conn.Subscribe("grlx.api.auth.users", func(msg *nats.Msg) {
		result := apitypes.UsersListResponse{
			Users: map[string]string{
				"KEY-ALPHA": "admin",
				"KEY-BETA":  "viewer",
			},
		}
		natsRespond(msg, result)
	})

	oldMode := outputMode
	defer func() { outputMode = oldMode }()
	outputMode = ""

	out := captureStdout(t, func() {
		usersListCmd.Run(usersListCmd, nil)
	})

	if !strings.Contains(out, "Users:") {
		t.Error("expected Users header")
	}
	if !strings.Contains(out, "admin") {
		t.Error("expected admin role in output")
	}
}

func TestUsersListCommand_NoUsers(t *testing.T) {
	conn, cleanup := setupTestNATS(t)
	defer cleanup()

	conn.Subscribe("grlx.api.auth.users", func(msg *nats.Msg) {
		result := apitypes.UsersListResponse{
			Users: map[string]string{},
		}
		natsRespond(msg, result)
	})

	oldMode := outputMode
	defer func() { outputMode = oldMode }()
	outputMode = ""

	out := captureStdout(t, func() {
		usersListCmd.Run(usersListCmd, nil)
	})

	if !strings.Contains(out, "No users configured") {
		t.Error("expected 'No users configured' message")
	}
}

func TestUsersListCommand_JSON(t *testing.T) {
	conn, cleanup := setupTestNATS(t)
	defer cleanup()

	conn.Subscribe("grlx.api.auth.users", func(msg *nats.Msg) {
		result := apitypes.UsersListResponse{
			Users: map[string]string{"KEY-1": "operator"},
		}
		natsRespond(msg, result)
	})

	oldMode := outputMode
	defer func() { outputMode = oldMode }()
	outputMode = "json"

	out := captureStdout(t, func() {
		usersListCmd.Run(usersListCmd, nil)
	})

	if !strings.Contains(out, "KEY-1") {
		t.Error("expected pubkey in JSON output")
	}
}

// --- Users add command ---

func TestUsersAddCommand_Text(t *testing.T) {
	conn, cleanup := setupTestNATS(t)
	defer cleanup()

	conn.Subscribe("grlx.api.auth.users.add", func(msg *nats.Msg) {
		result := apitypes.UserMutateResponse{
			Success: true,
			Message: "user added successfully",
		}
		natsRespond(msg, result)
	})

	oldMode := outputMode
	defer func() { outputMode = oldMode }()
	outputMode = ""

	out := captureStdout(t, func() {
		usersAddCmd.Run(usersAddCmd, []string{"operator", "NEWKEY123"})
	})

	if !strings.Contains(out, "user added successfully") {
		t.Error("expected success message in output")
	}
}

func TestUsersAddCommand_JSON(t *testing.T) {
	conn, cleanup := setupTestNATS(t)
	defer cleanup()

	conn.Subscribe("grlx.api.auth.users.add", func(msg *nats.Msg) {
		result := apitypes.UserMutateResponse{
			Success: true,
			Message: "added",
		}
		natsRespond(msg, result)
	})

	oldMode := outputMode
	defer func() { outputMode = oldMode }()
	outputMode = "json"

	out := captureStdout(t, func() {
		usersAddCmd.Run(usersAddCmd, []string{"viewer", "ADDKEY"})
	})

	if !strings.Contains(out, `"success":true`) {
		t.Error("expected success field in JSON")
	}
}

// --- Users remove command ---

func TestUsersRemoveCommand_Text(t *testing.T) {
	conn, cleanup := setupTestNATS(t)
	defer cleanup()

	conn.Subscribe("grlx.api.auth.users.remove", func(msg *nats.Msg) {
		result := apitypes.UserMutateResponse{
			Success: true,
			Message: "user removed",
		}
		natsRespond(msg, result)
	})

	oldMode := outputMode
	defer func() { outputMode = oldMode }()
	outputMode = ""

	out := captureStdout(t, func() {
		usersRemoveCmd.Run(usersRemoveCmd, []string{"DELKEY"})
	})

	if !strings.Contains(out, "user removed") {
		t.Error("expected removal message")
	}
}

func TestUsersRemoveCommand_JSON(t *testing.T) {
	conn, cleanup := setupTestNATS(t)
	defer cleanup()

	conn.Subscribe("grlx.api.auth.users.remove", func(msg *nats.Msg) {
		result := apitypes.UserMutateResponse{
			Success: true,
			Message: "removed",
		}
		natsRespond(msg, result)
	})

	oldMode := outputMode
	defer func() { outputMode = oldMode }()
	outputMode = "json"

	out := captureStdout(t, func() {
		usersRemoveCmd.Run(usersRemoveCmd, []string{"RMKEY"})
	})

	if !strings.Contains(out, `"success":true`) {
		t.Error("expected success field in JSON")
	}
}

// --- Recipes show JSON ---

func TestRecipesShowCommand_JSON(t *testing.T) {
	conn, cleanup := setupTestNATS(t)
	defer cleanup()

	conn.Subscribe("grlx.api.recipes.get", func(msg *nats.Msg) {
		result := RecipeContent{
			Name:    "base.packages",
			Path:    "/srv/recipes/base/packages.grlx",
			Content: "pkg.installed:\n  - name: nginx",
			Size:    42,
		}
		natsRespond(msg, result)
	})

	oldMode := outputMode
	defer func() { outputMode = oldMode }()
	outputMode = "json"

	out := captureStdout(t, func() {
		cmdRecipesShow.Run(cmdRecipesShow, []string{"base.packages"})
	})

	if !strings.Contains(out, "base.packages") {
		t.Error("expected recipe name in JSON output")
	}
	if !strings.Contains(out, "pkg.installed") {
		t.Error("expected recipe content in JSON output")
	}
}

// --- Cohorts show JSON ---

func TestCohortsShowCommand_JSON(t *testing.T) {
	conn, cleanup := setupTestNATS(t)
	defer cleanup()

	conn.Subscribe("grlx.api.cohorts.get", func(msg *nats.Msg) {
		result := client.CohortDetail{
			Name:    "db-servers",
			Type:    "dynamic",
			Members: []string{"db-1", "db-2"},
		}
		natsRespond(msg, result)
	})

	oldMode := outputMode
	defer func() { outputMode = oldMode }()
	outputMode = "json"

	out := captureStdout(t, func() {
		cmdCohortsShow.Run(cmdCohortsShow, []string{"db-servers"})
	})

	if !strings.Contains(out, "db-servers") {
		t.Error("expected cohort name in JSON output")
	}
}

// --- Cohorts list JSON ---

func TestCohortsListCommand_JSON(t *testing.T) {
	conn, cleanup := setupTestNATS(t)
	defer cleanup()

	conn.Subscribe("grlx.api.cohorts.list", func(msg *nats.Msg) {
		result := struct {
			Cohorts []struct {
				Name string `json:"name"`
				Type string `json:"type"`
			} `json:"cohorts"`
		}{
			Cohorts: []struct {
				Name string `json:"name"`
				Type string `json:"type"`
			}{
				{Name: "web-group", Type: "static"},
			},
		}
		natsRespond(msg, result)
	})

	oldMode := outputMode
	defer func() { outputMode = oldMode }()
	outputMode = "json"

	out := captureStdout(t, func() {
		cmdCohortsList.Run(cmdCohortsList, nil)
	})

	if !strings.Contains(out, "web-group") {
		t.Error("expected cohort name in JSON output")
	}
}

// --- Sprouts show with online status ---

func TestSproutsShowCommand_WithOnlineStatus(t *testing.T) {
	conn, cleanup := setupTestNATS(t)
	defer cleanup()

	conn.Subscribe("grlx.api.sprouts.get", func(msg *nats.Msg) {
		result := client.SproutInfo{
			ID:        "detailed-sprout",
			KeyState:  "accepted",
			Connected: true,
		}
		natsRespond(msg, result)
	})
	// Also subscribe to props endpoint (show command fetches it for JSON mode)
	conn.Subscribe("grlx.api.sprouts.props", func(msg *nats.Msg) {
		natsRespond(msg, map[string]any{})
	})

	oldMode := outputMode
	defer func() { outputMode = oldMode }()
	outputMode = ""

	out := captureStdout(t, func() {
		cmdSproutsShow.Run(cmdSproutsShow, []string{"detailed-sprout"})
	})

	if !strings.Contains(out, "detailed-sprout") {
		t.Error("expected sprout ID in output")
	}
	if !strings.Contains(out, "online") {
		t.Error("expected online status in output")
	}
}

func TestResolveEffectiveTarget_Cohort(t *testing.T) {
	conn, cleanup := setupTestNATS(t)
	defer cleanup()

	conn.Subscribe("grlx.api.cohorts.resolve", func(msg *nats.Msg) {
		result := struct {
			Sprouts []string `json:"sprouts"`
		}{
			Sprouts: []string{"web-1", "web-2"},
		}
		natsRespond(msg, result)
	})

	oldS, oldC := sproutTarget, cohortTarget
	defer func() { sproutTarget = oldS; cohortTarget = oldC }()

	sproutTarget = ""
	cohortTarget = "web-servers"

	out := captureStdout(t, func() {
		target, err := resolveEffectiveTarget()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if target != "web-1,web-2" {
			t.Errorf("expected 'web-1,web-2', got %q", target)
		}
	})

	if !strings.Contains(out, "web-servers") {
		t.Error("expected cohort name in resolution output")
	}
}
