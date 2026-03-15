package cmd

// Tests for the SSH picker model are in internal/sshpicker/picker_test.go
// to avoid the package init() in root.go which requires TLS setup.
//
// The resolveSSHTarget function is integration-tested via the CLI.
