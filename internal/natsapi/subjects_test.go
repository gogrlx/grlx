package natsapi

import (
	"strings"
	"testing"
)

func TestSubjectReturnsFullSubject(t *testing.T) {
	tests := []struct {
		method string
		want   string
	}{
		{MethodVersion, "grlx.api.version"},
		{MethodSproutsList, "grlx.api.sprouts.list"},
		{MethodPKIAccept, "grlx.api.pki.accept"},
		{MethodJobsCancel, "grlx.api.jobs.cancel"},
		{MethodAuthWhoAmI, "grlx.api.auth.whoami"},
		{MethodCook, "grlx.api.cook"},
		{MethodShellStart, "grlx.api.shell.start"},
		{MethodAuditQuery, "grlx.api.audit.query"},
	}

	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			got := Subject(tt.method)
			if got != tt.want {
				t.Errorf("Subject(%q) = %q, want %q", tt.method, got, tt.want)
			}
		})
	}
}

func TestSproutSubject(t *testing.T) {
	got := SproutSubject("web-01", SproutTestPing)
	want := "grlx.sprouts.web-01.test.ping"
	if got != want {
		t.Errorf("SproutSubject(web-01, test.ping) = %q, want %q", got, want)
	}

	got = SproutSubject("db-02", SproutCancel)
	want = "grlx.sprouts.db-02.cancel"
	if got != want {
		t.Errorf("SproutSubject(db-02, cancel) = %q, want %q", got, want)
	}

	got = SproutSubject("app-03", SproutShellStart)
	want = "grlx.sprouts.app-03.shell.start"
	if got != want {
		t.Errorf("SproutSubject(app-03, shell.start) = %q, want %q", got, want)
	}
}

func TestAllMethodsMatchRoutes(t *testing.T) {
	methods := AllMethods()
	if len(methods) != len(routes) {
		t.Errorf("AllMethods() has %d entries, routes map has %d", len(methods), len(routes))
	}

	for _, m := range methods {
		if _, ok := routes[m]; !ok {
			t.Errorf("AllMethods() includes %q but it's not in routes", m)
		}
	}

	for m := range routes {
		found := false
		for _, am := range methods {
			if am == m {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("routes contains %q but AllMethods() does not", m)
		}
	}
}

func TestAllMethodsHaveRBACMapping(t *testing.T) {
	for _, m := range AllMethods() {
		action := NATSMethodAction(m)
		// Every method should have an explicit mapping, not fall through to admin.
		if _, ok := natsActionMap[m]; !ok {
			t.Errorf("method %q has no explicit RBAC mapping (defaults to %s)", m, action)
		}
	}
}

func TestSubjectPrefixFormat(t *testing.T) {
	if !strings.HasSuffix(SubjectPrefix, ".") {
		t.Error("SubjectPrefix should end with a dot")
	}
	if !strings.HasSuffix(SproutSubjectPrefix, ".") {
		t.Error("SproutSubjectPrefix should end with a dot")
	}
}

func TestCookTriggerPrefix(t *testing.T) {
	jid := "20260326-abc123"
	got := SproutCookTriggerPrefix + jid
	want := "grlx.farmer.cook.trigger.20260326-abc123"
	if got != want {
		t.Errorf("cook trigger = %q, want %q", got, want)
	}
}
