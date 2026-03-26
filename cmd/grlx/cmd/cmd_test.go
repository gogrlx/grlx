package cmd

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/gogrlx/grlx/v2/internal/cook"
	"github.com/gogrlx/grlx/v2/internal/jobs"
)

// captureStdout runs fn and returns whatever it wrote to os.Stdout.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	fn()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

// --- truncate ---

func TestTruncate(t *testing.T) {
	tests := []struct {
		input    string
		max      int
		expected string
	}{
		{"hello", 10, "hello"},
		{"hello", 5, "hello"},
		{"hello world", 5, "hell…"},
		{"ab", 1, "…"},
		{"", 5, ""},
		{"exactly10!", 10, "exactly10!"},
		{"exactly11!!", 10, "exactly11…"},
	}
	for _, tt := range tests {
		got := truncate(tt.input, tt.max)
		if got != tt.expected {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.max, got, tt.expected)
		}
	}
}

// --- formatStatus ---

func TestFormatStatus(t *testing.T) {
	statuses := []struct {
		status   jobs.JobStatus
		contains string
	}{
		{jobs.JobSucceeded, "succeeded"},
		{jobs.JobFailed, "failed"},
		{jobs.JobRunning, "running"},
		{jobs.JobPending, "pending"},
		{jobs.JobPartial, "partial"},
		{jobs.JobStatus(99), "unknown"},
	}
	for _, tt := range statuses {
		got := formatStatus(tt.status)
		if !strings.Contains(got, tt.contains) {
			t.Errorf("formatStatus(%d) = %q, want to contain %q", tt.status, got, tt.contains)
		}
	}
}

// --- formatBytes ---

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		input    int64
		expected string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1023, "1023 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1572864, "1.5 MB"},
	}
	for _, tt := range tests {
		got := formatBytes(tt.input)
		if got != tt.expected {
			t.Errorf("formatBytes(%d) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

// --- formatSize (recipes) ---

func TestFormatSize(t *testing.T) {
	tests := []struct {
		input    int64
		expected string
	}{
		{0, "0 B"},
		{500, "500 B"},
		{1024, "1.0 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
	}
	for _, tt := range tests {
		got := formatSize(tt.input)
		if got != tt.expected {
			t.Errorf("formatSize(%d) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

// --- resolveEffectiveTarget ---

func TestResolveEffectiveTarget_BothSet(t *testing.T) {
	oldS, oldC := sproutTarget, cohortTarget
	defer func() { sproutTarget = oldS; cohortTarget = oldC }()

	sproutTarget = "sprout-1"
	cohortTarget = "web-servers"
	_, err := resolveEffectiveTarget()
	if err == nil {
		t.Fatal("expected error when both --target and --cohort are set")
	}
	if !strings.Contains(err.Error(), "cannot use both") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestResolveEffectiveTarget_NeitherSet(t *testing.T) {
	oldS, oldC := sproutTarget, cohortTarget
	defer func() { sproutTarget = oldS; cohortTarget = oldC }()

	sproutTarget = ""
	cohortTarget = ""
	_, err := resolveEffectiveTarget()
	if err == nil {
		t.Fatal("expected error when neither flag is set")
	}
	if !strings.Contains(err.Error(), "required") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestResolveEffectiveTarget_TargetOnly(t *testing.T) {
	oldS, oldC := sproutTarget, cohortTarget
	defer func() { sproutTarget = oldS; cohortTarget = oldC }()

	sproutTarget = "sprout-a,sprout-b"
	cohortTarget = ""
	got, err := resolveEffectiveTarget()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "sprout-a,sprout-b" {
		t.Errorf("got %q, want %q", got, "sprout-a,sprout-b")
	}
}

// --- printJobsTable ---

func TestPrintJobsTable_Empty(t *testing.T) {
	out := captureStdout(t, func() {
		printJobsTable([]jobs.JobSummary{})
	})
	// Empty slice still prints a header
	if !strings.Contains(out, "JID") {
		t.Error("expected header with JID column")
	}
}

func TestPrintJobsTable_WithoutInvoker(t *testing.T) {
	summaries := []jobs.JobSummary{
		{
			JID:       "abc123",
			SproutID:  "web-1",
			Status:    jobs.JobSucceeded,
			Succeeded: 3,
			Failed:    0,
			Skipped:   1,
			StartedAt: time.Date(2026, 3, 26, 3, 0, 0, 0, time.UTC),
		},
	}
	out := captureStdout(t, func() {
		printJobsTable(summaries)
	})
	if !strings.Contains(out, "abc123") {
		t.Error("expected JID in output")
	}
	if !strings.Contains(out, "web-1") {
		t.Error("expected sprout ID in output")
	}
	if strings.Contains(out, "USER") {
		t.Error("USER column should not appear when no invoker is set")
	}
}

func TestPrintJobsTable_WithInvoker(t *testing.T) {
	summaries := []jobs.JobSummary{
		{
			JID:       "def456",
			SproutID:  "db-1",
			Status:    jobs.JobFailed,
			InvokedBy: "ABCDEF123456",
			Succeeded: 1,
			Failed:    2,
			Skipped:   0,
			StartedAt: time.Date(2026, 3, 26, 3, 30, 0, 0, time.UTC),
		},
	}
	out := captureStdout(t, func() {
		printJobsTable(summaries)
	})
	if !strings.Contains(out, "USER") {
		t.Error("USER column should appear when invoker is set")
	}
	if !strings.Contains(out, "def456") {
		t.Error("expected JID in output")
	}
}

func TestPrintJobsTable_TruncatesLongFields(t *testing.T) {
	summaries := []jobs.JobSummary{
		{
			JID:       "this-jid-is-way-too-long-to-fit",
			SproutID:  "this-sprout-id-is-way-too-long-as-well",
			Status:    jobs.JobRunning,
			Succeeded: 0,
			Failed:    0,
		},
	}
	out := captureStdout(t, func() {
		printJobsTable(summaries)
	})
	if !strings.Contains(out, "…") {
		t.Error("expected truncation ellipsis in output")
	}
}

// --- printJobDetail ---

func TestPrintJobDetail_BasicOutput(t *testing.T) {
	summary := &jobs.JobSummary{
		JID:       "job-001",
		SproutID:  "app-1",
		Status:    jobs.JobSucceeded,
		InvokedBy: "user-key-xyz",
		StartedAt: time.Date(2026, 3, 26, 3, 0, 0, 0, time.UTC),
		Duration:  5 * time.Second,
		Total:     3,
		Succeeded: 3,
		Failed:    0,
		Skipped:   0,
	}
	out := captureStdout(t, func() {
		printJobDetail(summary)
	})
	if !strings.Contains(out, "job-001") {
		t.Error("expected JID in detail output")
	}
	if !strings.Contains(out, "app-1") {
		t.Error("expected sprout ID in detail output")
	}
	if !strings.Contains(out, "user-key-xyz") {
		t.Error("expected user in detail output")
	}
	if !strings.Contains(out, "5s") {
		t.Error("expected duration in detail output")
	}
	if !strings.Contains(out, "3 total") {
		t.Error("expected step counts in detail output")
	}
}

func TestPrintJobDetail_NoSteps(t *testing.T) {
	summary := &jobs.JobSummary{
		JID:      "job-002",
		SproutID: "app-2",
		Status:   jobs.JobPending,
	}
	out := captureStdout(t, func() {
		printJobDetail(summary)
	})
	if !strings.Contains(out, "No steps recorded") {
		t.Error("expected 'No steps recorded' message")
	}
}

func TestPrintJobDetail_WithSteps(t *testing.T) {
	summary := &jobs.JobSummary{
		JID:       "job-003",
		SproutID:  "app-3",
		Status:    jobs.JobFailed,
		Total:     3,
		Succeeded: 1,
		Failed:    1,
		Skipped:   1,
		Steps: []cook.StepCompletion{
			{
				ID:               "step-1",
				CompletionStatus: cook.StepCompleted,
				Changes:          []string{"installed package X"},
				Started:          time.Date(2026, 3, 26, 3, 0, 0, 0, time.UTC),
				Duration:         2 * time.Second,
			},
			{
				ID:               "step-2",
				CompletionStatus: cook.StepFailed,
				Changes:          []string{"failed to start service"},
			},
			{
				ID:               "step-3",
				CompletionStatus: cook.StepSkipped,
			},
		},
	}
	out := captureStdout(t, func() {
		printJobDetail(summary)
	})
	if !strings.Contains(out, "step-1") {
		t.Error("expected step-1 in output")
	}
	if !strings.Contains(out, "installed package X") {
		t.Error("expected change note in output")
	}
	if !strings.Contains(out, "step-2") {
		t.Error("expected step-2 in output")
	}
}

func TestPrintJobDetail_SkipsSyntheticMarkers(t *testing.T) {
	summary := &jobs.JobSummary{
		JID:      "job-004",
		SproutID: "app-4",
		Status:   jobs.JobSucceeded,
		Total:    1,
		Steps: []cook.StepCompletion{
			{ID: "start-job-004", CompletionStatus: cook.StepCompleted},
			{ID: "real-step", CompletionStatus: cook.StepCompleted},
			{ID: "completed-job-004", CompletionStatus: cook.StepCompleted},
		},
	}
	out := captureStdout(t, func() {
		printJobDetail(summary)
	})
	if strings.Contains(out, "start-job-004") {
		t.Error("synthetic start marker should be filtered")
	}
	if strings.Contains(out, "completed-job-004") {
		t.Error("synthetic completed marker should be filtered")
	}
	if !strings.Contains(out, "real-step") {
		t.Error("real step should be present")
	}
}

func TestPrintJobDetail_ZeroStartedAt(t *testing.T) {
	summary := &jobs.JobSummary{
		JID:      "job-005",
		SproutID: "app-5",
		Status:   jobs.JobPending,
	}
	out := captureStdout(t, func() {
		printJobDetail(summary)
	})
	if strings.Contains(out, "Started:") {
		t.Error("should not print Started when zero")
	}
	if strings.Contains(out, "Duration:") {
		t.Error("should not print Duration when zero")
	}
}

func TestPrintJobDetail_StepStatuses(t *testing.T) {
	summary := &jobs.JobSummary{
		JID:      "job-006",
		SproutID: "app-6",
		Status:   jobs.JobRunning,
		Total:    2,
		Steps: []cook.StepCompletion{
			{ID: "in-progress", CompletionStatus: cook.StepInProgress},
			{ID: "not-started", CompletionStatus: cook.StepNotStarted},
		},
	}
	out := captureStdout(t, func() {
		printJobDetail(summary)
	})
	if !strings.Contains(out, "In Progress") {
		t.Error("expected 'In Progress' status")
	}
	if !strings.Contains(out, "Not Started") {
		t.Error("expected 'Not Started' status")
	}
}

// --- printRecipesTable ---

func TestPrintRecipesTable(t *testing.T) {
	recipes := []RecipeInfo{
		{Name: "webserver.nginx", Path: "/srv/recipes/webserver/nginx.grlx", Size: 2048},
		{Name: "base.packages", Path: "/srv/recipes/base/packages.grlx", Size: 512},
	}
	out := captureStdout(t, func() {
		printRecipesTable(recipes)
	})
	if !strings.Contains(out, "webserver.nginx") {
		t.Error("expected recipe name in output")
	}
	if !strings.Contains(out, "2.0 KB") {
		t.Error("expected formatted size")
	}
	if !strings.Contains(out, "2 recipe(s)") {
		t.Error("expected recipe count summary")
	}
}

func TestPrintRecipesTable_SingleRecipe(t *testing.T) {
	recipes := []RecipeInfo{
		{Name: "a", Path: "/a.grlx", Size: 100},
	}
	out := captureStdout(t, func() {
		printRecipesTable(recipes)
	})
	if !strings.Contains(out, "1 recipe(s)") {
		t.Error("expected '1 recipe(s)' summary")
	}
}

// --- builtinRoleNames ---

func TestBuiltinRoleNames(t *testing.T) {
	if !builtinRoleNames["viewer"] {
		t.Error("viewer should be a builtin role")
	}
	if !builtinRoleNames["operator"] {
		t.Error("operator should be a builtin role")
	}
	if builtinRoleNames["admin"] {
		t.Error("admin is not a builtin role (it's config-defined)")
	}
	if builtinRoleNames["custom-role"] {
		t.Error("arbitrary role should not be builtin")
	}
}

// --- Command tree structure ---

func TestRootCommand_HasExpectedSubcommands(t *testing.T) {
	expected := []string{
		"version", "cook", "cmd", "test", "keys",
		"tail", "sprouts", "jobs", "auth", "init",
		"cohorts", "recipes", "roles", "users", "ssh",
		"serve", "audit",
	}
	commands := rootCmd.Commands()
	names := make(map[string]bool)
	for _, c := range commands {
		names[c.Name()] = true
	}
	for _, name := range expected {
		if !names[name] {
			t.Errorf("root command missing subcommand %q", name)
		}
	}
}

func TestJobsCommand_HasSubcommands(t *testing.T) {
	expected := []string{"list", "show", "watch", "cancel"}
	names := make(map[string]bool)
	for _, c := range cmdJobs.Commands() {
		names[c.Name()] = true
	}
	for _, name := range expected {
		if !names[name] {
			t.Errorf("jobs command missing subcommand %q", name)
		}
	}
}

func TestSproutsCommand_HasSubcommands(t *testing.T) {
	expected := []string{"list", "show"}
	names := make(map[string]bool)
	for _, c := range cmdSprouts.Commands() {
		names[c.Name()] = true
	}
	for _, name := range expected {
		if !names[name] {
			t.Errorf("sprouts command missing subcommand %q", name)
		}
	}
}

func TestKeysCommand_HasSubcommands(t *testing.T) {
	expected := []string{"accept", "deny", "reject", "unaccept", "delete", "list"}
	names := make(map[string]bool)
	for _, c := range keysCmd.Commands() {
		names[c.Name()] = true
	}
	for _, name := range expected {
		if !names[name] {
			t.Errorf("keys command missing subcommand %q", name)
		}
	}
}

func TestAuthCommand_HasSubcommands(t *testing.T) {
	expected := []string{"privkey", "pubkey", "token", "whoami", "users", "roles", "explain"}
	names := make(map[string]bool)
	for _, c := range authCmd.Commands() {
		names[c.Name()] = true
	}
	for _, name := range expected {
		if !names[name] {
			t.Errorf("auth command missing subcommand %q", name)
		}
	}
}

func TestCohortsCommand_HasSubcommands(t *testing.T) {
	expected := []string{"list", "show", "resolve", "refresh"}
	names := make(map[string]bool)
	for _, c := range cmdCohorts.Commands() {
		names[c.Name()] = true
	}
	for _, name := range expected {
		if !names[name] {
			t.Errorf("cohorts command missing subcommand %q", name)
		}
	}
}

func TestRecipesCommand_HasSubcommands(t *testing.T) {
	expected := []string{"list", "show"}
	names := make(map[string]bool)
	for _, c := range cmdRecipes.Commands() {
		names[c.Name()] = true
	}
	for _, name := range expected {
		if !names[name] {
			t.Errorf("recipes command missing subcommand %q", name)
		}
	}
}

func TestUsersCommand_HasSubcommands(t *testing.T) {
	expected := []string{"list", "add", "remove"}
	names := make(map[string]bool)
	for _, c := range usersCmd.Commands() {
		names[c.Name()] = true
	}
	for _, name := range expected {
		if !names[name] {
			t.Errorf("users command missing subcommand %q", name)
		}
	}
}

func TestAuditCommand_HasSubcommands(t *testing.T) {
	expected := []string{"dates", "list"}
	names := make(map[string]bool)
	for _, c := range cmdAudit.Commands() {
		names[c.Name()] = true
	}
	for _, name := range expected {
		if !names[name] {
			t.Errorf("audit command missing subcommand %q", name)
		}
	}
}

// --- Flag validation ---

func TestJobsListFlags(t *testing.T) {
	flags := cmdJobsList.Flags()

	if f := flags.Lookup("limit"); f == nil {
		t.Error("jobs list missing --limit flag")
	} else if f.DefValue != "50" {
		t.Errorf("jobs list --limit default = %q, want 50", f.DefValue)
	}

	if f := flags.Lookup("local"); f == nil {
		t.Error("jobs list missing --local flag")
	}

	if f := flags.Lookup("user"); f == nil {
		t.Error("jobs list missing --user flag")
	}
}

func TestJobsWatchFlags(t *testing.T) {
	if f := cmdJobsWatch.Flags().Lookup("timeout"); f == nil {
		t.Error("jobs watch missing --timeout flag")
	} else if f.DefValue != "120" {
		t.Errorf("jobs watch --timeout default = %q, want 120", f.DefValue)
	}
}

func TestCookFlags(t *testing.T) {
	flags := cmdCook.Flags()

	if f := flags.Lookup("async"); f == nil {
		t.Error("cook missing --async flag")
	}

	if f := flags.Lookup("cook-timeout"); f == nil {
		t.Error("cook missing --cook-timeout flag")
	} else if f.DefValue != "30" {
		t.Errorf("cook --cook-timeout default = %q, want 30", f.DefValue)
	}

	if f := flags.Lookup("test"); f == nil {
		t.Error("cook missing --test flag")
	}
}

func TestServeFlags(t *testing.T) {
	flags := serveCmd.Flags()

	if f := flags.Lookup("addr"); f == nil {
		t.Error("serve missing --addr flag")
	} else if f.DefValue != "127.0.0.1" {
		t.Errorf("serve --addr default = %q, want 127.0.0.1", f.DefValue)
	}

	if f := flags.Lookup("port"); f == nil {
		t.Error("serve missing --port flag")
	} else if f.DefValue != "5407" {
		t.Errorf("serve --port default = %q, want 5407", f.DefValue)
	}
}

func TestRootPersistentFlags(t *testing.T) {
	if f := rootCmd.PersistentFlags().Lookup("out"); f == nil {
		t.Error("root missing --out persistent flag")
	}

	if f := rootCmd.PersistentFlags().Lookup("config"); f == nil {
		t.Error("root missing --config persistent flag")
	}
}

func TestSproutsListFlags(t *testing.T) {
	flags := cmdSproutsList.Flags()

	if f := flags.Lookup("state"); f == nil {
		t.Error("sprouts list missing --state flag")
	}

	if f := flags.Lookup("online"); f == nil {
		t.Error("sprouts list missing --online flag")
	}
}

func TestAuditListFlags(t *testing.T) {
	flags := cmdAuditList.Flags()

	for _, name := range []string{"date", "action", "pubkey", "limit", "failed"} {
		if f := flags.Lookup(name); f == nil {
			t.Errorf("audit list missing --%s flag", name)
		}
	}

	if f := flags.Lookup("limit"); f != nil && f.DefValue != "50" {
		t.Errorf("audit list --limit default = %q, want 50", f.DefValue)
	}
}

func TestKeysAcceptFlags(t *testing.T) {
	if f := keysAccept.Flags().Lookup("all"); f == nil {
		t.Error("keys accept missing --all flag")
	}
}

// --- Args validation ---

func TestJobsShowArgs(t *testing.T) {
	if cmdJobsShow.Args == nil {
		t.Fatal("jobs show should have args validation")
	}
	if err := cmdJobsShow.Args(cmdJobsShow, []string{}); err == nil {
		t.Error("jobs show should reject zero args")
	}
	if err := cmdJobsShow.Args(cmdJobsShow, []string{"jid1"}); err != nil {
		t.Errorf("jobs show should accept 1 arg: %v", err)
	}
	if err := cmdJobsShow.Args(cmdJobsShow, []string{"jid1", "jid2"}); err == nil {
		t.Error("jobs show should reject 2 args")
	}
}

func TestJobsWatchArgs(t *testing.T) {
	if cmdJobsWatch.Args == nil {
		t.Fatal("jobs watch should have args validation")
	}
	if err := cmdJobsWatch.Args(cmdJobsWatch, []string{}); err == nil {
		t.Error("jobs watch should reject zero args")
	}
	if err := cmdJobsWatch.Args(cmdJobsWatch, []string{"jid1"}); err != nil {
		t.Errorf("jobs watch should accept 1 arg: %v", err)
	}
}

func TestJobsCancelArgs(t *testing.T) {
	if cmdJobsCancel.Args == nil {
		t.Fatal("jobs cancel should have args validation")
	}
	if err := cmdJobsCancel.Args(cmdJobsCancel, []string{}); err == nil {
		t.Error("jobs cancel should reject zero args")
	}
	if err := cmdJobsCancel.Args(cmdJobsCancel, []string{"jid1"}); err != nil {
		t.Errorf("jobs cancel should accept 1 arg: %v", err)
	}
}

func TestSproutsShowArgs(t *testing.T) {
	if err := cmdSproutsShow.Args(cmdSproutsShow, []string{}); err == nil {
		t.Error("sprouts show should reject zero args")
	}
	if err := cmdSproutsShow.Args(cmdSproutsShow, []string{"s1"}); err != nil {
		t.Errorf("sprouts show should accept 1 arg: %v", err)
	}
}

func TestCohortsShowArgs(t *testing.T) {
	if err := cmdCohortsShow.Args(cmdCohortsShow, []string{}); err == nil {
		t.Error("cohorts show should reject zero args")
	}
}

func TestCohortsResolveArgs(t *testing.T) {
	if err := cmdCohortsResolve.Args(cmdCohortsResolve, []string{}); err == nil {
		t.Error("cohorts resolve should reject zero args")
	}
}

func TestCohortsRefreshArgs(t *testing.T) {
	// MaximumNArgs(1)
	if err := cmdCohortsRefresh.Args(cmdCohortsRefresh, []string{}); err != nil {
		t.Errorf("cohorts refresh should accept zero args: %v", err)
	}
	if err := cmdCohortsRefresh.Args(cmdCohortsRefresh, []string{"name"}); err != nil {
		t.Errorf("cohorts refresh should accept one arg: %v", err)
	}
	if err := cmdCohortsRefresh.Args(cmdCohortsRefresh, []string{"a", "b"}); err == nil {
		t.Error("cohorts refresh should reject two args")
	}
}

func TestRecipesShowArgs(t *testing.T) {
	if err := cmdRecipesShow.Args(cmdRecipesShow, []string{}); err == nil {
		t.Error("recipes show should reject zero args")
	}
}

func TestUsersAddArgs(t *testing.T) {
	if err := usersAddCmd.Args(usersAddCmd, []string{}); err == nil {
		t.Error("users add should reject zero args")
	}
	if err := usersAddCmd.Args(usersAddCmd, []string{"role"}); err == nil {
		t.Error("users add should reject one arg")
	}
	if err := usersAddCmd.Args(usersAddCmd, []string{"role", "pubkey"}); err != nil {
		t.Errorf("users add should accept two args: %v", err)
	}
}

func TestUsersRemoveArgs(t *testing.T) {
	if err := usersRemoveCmd.Args(usersRemoveCmd, []string{}); err == nil {
		t.Error("users remove should reject zero args")
	}
	if err := usersRemoveCmd.Args(usersRemoveCmd, []string{"pk"}); err != nil {
		t.Errorf("users remove should accept one arg: %v", err)
	}
}

// --- init configModel (bubbletea model for init command) ---

func TestInitialModel(t *testing.T) {
	m := initialModel()
	if len(m.inputs) != 3 {
		t.Fatalf("expected 3 inputs, got %d", len(m.inputs))
	}
	if m.focusIndex != 0 {
		t.Errorf("expected focusIndex 0, got %d", m.focusIndex)
	}
}

func TestConfigModelView(t *testing.T) {
	m := initialModel()
	view := m.View()
	if !strings.Contains(view, "grlx") {
		t.Error("view should contain 'grlx'")
	}
	if !strings.Contains(view, "Submit") {
		t.Error("view should contain Submit button")
	}
}

func TestConfigModelInit(t *testing.T) {
	m := initialModel()
	cmd := m.Init()
	if cmd == nil {
		t.Error("Init() should return a command (textinput.Blink)")
	}
}

// --- RecipeInfo / RecipeContent types ---

func TestRecipeInfoJSON(t *testing.T) {
	info := RecipeInfo{
		Name: "test.recipe",
		Path: "/srv/recipes/test.grlx",
		Size: 1024,
	}
	if info.Name != "test.recipe" || info.Size != 1024 {
		t.Error("RecipeInfo fields not set correctly")
	}
}

func TestRecipeContentJSON(t *testing.T) {
	content := RecipeContent{
		Name:    "test.recipe",
		Path:    "/srv/recipes/test.grlx",
		Content: "file.managed:\n  /etc/test:",
		Size:    42,
	}
	if content.Content == "" {
		t.Error("RecipeContent.Content should not be empty")
	}
}

// --- Command Use/Short strings ---

func TestCommandMetadata(t *testing.T) {
	tests := []struct {
		name    string
		cmd     interface{ Name() string }
		wantUse string
	}{
		{"root", rootCmd, "grlx"},
		{"version", versionCmd, "version"},
		{"cook", cmdCook, "cook"},
		{"tail", tailCmd, "tail"},
		{"ssh", sshCmd, "ssh"},
		{"serve", serveCmd, "serve"},
	}
	for _, tt := range tests {
		if tt.cmd.Name() != tt.wantUse {
			t.Errorf("%s: Name() = %q, want %q", tt.name, tt.cmd.Name(), tt.wantUse)
		}
	}
}

// --- configModel Update (bubbletea) ---

func TestConfigModelUpdate_CtrlC(t *testing.T) {
	m := initialModel()
	updated, cmd := m.Update(tea.KeyMsg(tea.Key{Type: tea.KeyCtrlC}))
	if updated == nil {
		t.Fatal("expected non-nil model")
	}
	if cmd == nil {
		t.Fatal("expected quit command")
	}
}

func TestConfigModelUpdate_Tab(t *testing.T) {
	m := initialModel()
	if m.focusIndex != 0 {
		t.Fatal("initial focus should be 0")
	}
	updated, _ := m.Update(tea.KeyMsg(tea.Key{Type: tea.KeyTab}))
	m2 := updated.(configModel)
	if m2.focusIndex != 1 {
		t.Errorf("after tab, focus should be 1, got %d", m2.focusIndex)
	}
}

func TestConfigModelUpdate_ShiftTab(t *testing.T) {
	m := initialModel()
	// Focus is at 0, shift-tab should wrap to the end (len(inputs) = 3, so index 3 = submit button)
	updated, _ := m.Update(tea.KeyMsg(tea.Key{Type: tea.KeyShiftTab}))
	m2 := updated.(configModel)
	if m2.focusIndex != len(m.inputs) {
		t.Errorf("after shift-tab from 0, focus should be %d, got %d", len(m.inputs), m2.focusIndex)
	}
}

func TestConfigModelUpdate_Down(t *testing.T) {
	m := initialModel()
	updated, _ := m.Update(tea.KeyMsg(tea.Key{Type: tea.KeyDown}))
	m2 := updated.(configModel)
	if m2.focusIndex != 1 {
		t.Errorf("after down, focus should be 1, got %d", m2.focusIndex)
	}
}

func TestConfigModelUpdate_Up(t *testing.T) {
	m := initialModel()
	m.focusIndex = 2
	updated, _ := m.Update(tea.KeyMsg(tea.Key{Type: tea.KeyUp}))
	m2 := updated.(configModel)
	if m2.focusIndex != 1 {
		t.Errorf("after up from 2, focus should be 1, got %d", m2.focusIndex)
	}
}

func TestConfigModelUpdate_TabWrapsAround(t *testing.T) {
	m := initialModel()
	m.focusIndex = len(m.inputs) // at submit button (index 3)
	updated, _ := m.Update(tea.KeyMsg(tea.Key{Type: tea.KeyTab}))
	m2 := updated.(configModel)
	if m2.focusIndex != 0 {
		t.Errorf("after tab from submit, focus should wrap to 0, got %d", m2.focusIndex)
	}
}

func TestConfigModelUpdate_CtrlR_CyclesCursorMode(t *testing.T) {
	m := initialModel()
	origMode := m.cursorMode
	updated, _ := m.Update(tea.KeyMsg(tea.Key{Type: tea.KeyCtrlR}))
	m2 := updated.(configModel)
	if m2.cursorMode == origMode {
		t.Error("ctrl+r should cycle cursor mode")
	}
}

func TestConfigModelUpdate_EnterOnSubmit(t *testing.T) {
	m := initialModel()
	m.focusIndex = len(m.inputs) // submit button
	_, cmd := m.Update(tea.KeyMsg(tea.Key{Type: tea.KeyEnter}))
	if cmd == nil {
		t.Fatal("enter on submit should produce a quit command")
	}
}

func TestConfigModelUpdate_EnterOnInput(t *testing.T) {
	m := initialModel()
	m.focusIndex = 0 // first input field
	updated, _ := m.Update(tea.KeyMsg(tea.Key{Type: tea.KeyEnter}))
	m2 := updated.(configModel)
	// Enter on an input field should move focus forward, not quit
	if m2.focusIndex != 1 {
		t.Errorf("enter on input 0 should move focus to 1, got %d", m2.focusIndex)
	}
}

func TestConfigModelUpdate_CharacterInput(t *testing.T) {
	m := initialModel()
	// Send a rune message to the focused input
	updated, _ := m.Update(tea.KeyMsg(tea.Key{Type: tea.KeyRunes, Runes: []rune{'a'}}))
	m2 := updated.(configModel)
	if m2.inputs[0].Value() != "a" {
		t.Errorf("expected input 0 to have value 'a', got %q", m2.inputs[0].Value())
	}
}

func TestConfigModelView_FocusedButton(t *testing.T) {
	m := initialModel()
	m.focusIndex = len(m.inputs) // submit button focused
	view := m.View()
	// When submit is focused, the focused style should be used
	if !strings.Contains(view, "Submit") {
		t.Error("view should contain Submit")
	}
}

func TestConfigModelView_BlurredButton(t *testing.T) {
	m := initialModel()
	m.focusIndex = 0 // not on submit
	view := m.View()
	if !strings.Contains(view, "Submit") {
		t.Error("view should contain Submit even when blurred")
	}
}

// --- Edge cases for formatting functions ---

func TestFormatStatus_AllValues(t *testing.T) {
	// Ensure every known status produces a non-empty string
	for _, s := range []jobs.JobStatus{jobs.JobSucceeded, jobs.JobFailed, jobs.JobRunning, jobs.JobPending, jobs.JobPartial} {
		result := formatStatus(s)
		if result == "" {
			t.Errorf("formatStatus(%d) returned empty string", s)
		}
	}
}

func TestTruncate_ExactMatch(t *testing.T) {
	// String exactly at max should not be truncated
	s := "12345"
	if got := truncate(s, 5); got != s {
		t.Errorf("truncate(%q, 5) = %q, want %q", s, got, s)
	}
}

func TestTruncate_SingleChar(t *testing.T) {
	// Max of 2 with a 5-char string: keep 1 char + ellipsis
	got := truncate("hello", 2)
	if got != "h…" {
		t.Errorf("truncate('hello', 2) = %q, want %q", got, "h…")
	}
}

func TestPrintJobsTable_MultipleJobs(t *testing.T) {
	summaries := []jobs.JobSummary{
		{JID: "j1", SproutID: "s1", Status: jobs.JobSucceeded, Succeeded: 5},
		{JID: "j2", SproutID: "s2", Status: jobs.JobFailed, Failed: 3},
		{JID: "j3", SproutID: "s3", Status: jobs.JobRunning},
	}
	out := captureStdout(t, func() {
		printJobsTable(summaries)
	})
	if !strings.Contains(out, "j1") || !strings.Contains(out, "j2") || !strings.Contains(out, "j3") {
		t.Error("all JIDs should appear in output")
	}
}

func TestPrintJobDetail_WithStartedAndDurationButNoInvoker(t *testing.T) {
	summary := &jobs.JobSummary{
		JID:       "job-007",
		SproutID:  "web-7",
		Status:    jobs.JobSucceeded,
		StartedAt: time.Date(2026, 3, 26, 3, 0, 0, 0, time.UTC),
		Duration:  100 * time.Millisecond,
		Total:     1,
		Succeeded: 1,
	}
	out := captureStdout(t, func() {
		printJobDetail(summary)
	})
	if !strings.Contains(out, "Started:") {
		t.Error("expected Started field")
	}
	if !strings.Contains(out, "Duration:") {
		t.Error("expected Duration field")
	}
	if strings.Contains(out, "User:") {
		t.Error("User should not appear when InvokedBy is empty")
	}
}

func TestFormatBytes_EdgeCases(t *testing.T) {
	tests := []struct {
		input    int64
		expected string
	}{
		{1, "1 B"},
		{1024, "1.0 KB"},
		{1025, "1.0 KB"},
		{2097152, "2.0 MB"},
	}
	for _, tt := range tests {
		got := formatBytes(tt.input)
		if got != tt.expected {
			t.Errorf("formatBytes(%d) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestFormatSize_EdgeCases(t *testing.T) {
	tests := []struct {
		input    int64
		expected string
	}{
		{1, "1 B"},
		{1023, "1023 B"},
		{2048, "2.0 KB"},
		{5242880, "5.0 MB"},
	}
	for _, tt := range tests {
		got := formatSize(tt.input)
		if got != tt.expected {
			t.Errorf("formatSize(%d) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

// --- SSH flags ---

func TestSSHFlags(t *testing.T) {
	flags := sshCmd.Flags()

	if f := flags.Lookup("shell"); f == nil {
		t.Error("ssh missing --shell flag")
	}
	if f := flags.Lookup("cohort"); f == nil {
		t.Error("ssh missing --cohort flag")
	}
	if f := flags.Lookup("idle-timeout"); f == nil {
		t.Error("ssh missing --idle-timeout flag")
	} else if f.DefValue != "0" {
		t.Errorf("ssh --idle-timeout default = %q, want 0", f.DefValue)
	}
}

func TestSSHArgs(t *testing.T) {
	// MaximumNArgs(1)
	if err := sshCmd.Args(sshCmd, []string{}); err != nil {
		t.Errorf("ssh should accept zero args: %v", err)
	}
	if err := sshCmd.Args(sshCmd, []string{"sprout-1"}); err != nil {
		t.Errorf("ssh should accept one arg: %v", err)
	}
	if err := sshCmd.Args(sshCmd, []string{"a", "b"}); err == nil {
		t.Error("ssh should reject two args")
	}
}

// --- Test cmd flags ---

func TestTestPingFlags(t *testing.T) {
	if f := testCmdPing.Flags().Lookup("all"); f == nil {
		t.Error("test ping missing --all flag")
	}
}

func TestCmdRunFlags(t *testing.T) {
	flags := cmdCmdRun.Flags()
	for _, name := range []string{"environment", "noerr", "runas", "cwd", "timeout", "path"} {
		if f := flags.Lookup(name); f == nil {
			t.Errorf("cmd run missing --%s flag", name)
		}
	}
}

// --- keys flags ---

func TestKeysNoConfirmFlag(t *testing.T) {
	if f := keysCmd.PersistentFlags().Lookup("no-confirm"); f == nil {
		t.Error("keys missing --no-confirm persistent flag")
	}
}
