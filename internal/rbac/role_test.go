package rbac

import (
	"testing"
)

func TestIsValidRole(t *testing.T) {
	tests := []struct {
		role  Role
		valid bool
	}{
		{RoleAdmin, true},
		{RoleOperator, true},
		{RoleViewer, true},
		{Role("superuser"), false},
		{Role(""), false},
	}
	for _, tt := range tests {
		if got := IsValidRole(tt.role); got != tt.valid {
			t.Errorf("IsValidRole(%q) = %v, want %v", tt.role, got, tt.valid)
		}
	}
}

func TestParseRole(t *testing.T) {
	tests := []struct {
		input string
		want  Role
		err   bool
	}{
		{"admin", RoleAdmin, false},
		{"operator", RoleOperator, false},
		{"viewer", RoleViewer, false},
		{"root", "", true},
		{"", "", true},
	}
	for _, tt := range tests {
		got, err := ParseRole(tt.input)
		if (err != nil) != tt.err {
			t.Errorf("ParseRole(%q) error = %v, wantErr %v", tt.input, err, tt.err)
			continue
		}
		if got != tt.want {
			t.Errorf("ParseRole(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestRoleHasAccess(t *testing.T) {
	tests := []struct {
		role  Role
		route string
		want  bool
	}{
		// Viewer access
		{RoleViewer, "GetVersion", true},
		{RoleViewer, "ListSprouts", true},
		{RoleViewer, "ListJobs", true},
		{RoleViewer, "GetAllProps", true},

		// Viewer cannot cook or manage PKI
		{RoleViewer, "Cook", false},
		{RoleViewer, "CmdRun", false},
		{RoleViewer, "AcceptID", false},
		{RoleViewer, "DeleteID", false},

		// Operator access
		{RoleOperator, "GetVersion", true},
		{RoleOperator, "ListSprouts", true},
		{RoleOperator, "Cook", true},
		{RoleOperator, "CmdRun", true},
		{RoleOperator, "TestPing", true},
		{RoleOperator, "SetProp", true},
		{RoleOperator, "CancelJob", true},

		// Operator cannot manage PKI
		{RoleOperator, "AcceptID", false},
		{RoleOperator, "RejectID", false},
		{RoleOperator, "DeleteID", false},

		// Admin access
		{RoleAdmin, "GetVersion", true},
		{RoleAdmin, "Cook", true},
		{RoleAdmin, "AcceptID", true},
		{RoleAdmin, "DeleteID", true},

		// Unknown route defaults to admin-only
		{RoleViewer, "SomeFutureRoute", false},
		{RoleOperator, "SomeFutureRoute", false},
		{RoleAdmin, "SomeFutureRoute", true},
	}
	for _, tt := range tests {
		got := RoleHasAccess(tt.role, tt.route)
		if got != tt.want {
			t.Errorf("RoleHasAccess(%q, %q) = %v, want %v", tt.role, tt.route, got, tt.want)
		}
	}
}

func TestRequiredRole(t *testing.T) {
	tests := []struct {
		route string
		want  Role
	}{
		{"GetVersion", RoleViewer},
		{"Cook", RoleOperator},
		{"AcceptID", RoleAdmin},
		{"UnknownRoute", RoleAdmin},
	}
	for _, tt := range tests {
		got := RequiredRole(tt.route)
		if got != tt.want {
			t.Errorf("RequiredRole(%q) = %q, want %q", tt.route, got, tt.want)
		}
	}
}
