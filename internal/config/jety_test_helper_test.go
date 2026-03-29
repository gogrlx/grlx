package config

import (
	"testing"
	_ "unsafe" // required for go:linkname

	"github.com/taigrr/jety"
)

//go:linkname defaultConfigManager github.com/taigrr/jety.defaultConfigManager
var defaultConfigManager *jety.ConfigManager

// resetJety replaces jety's global default config manager with a fresh
// instance, ensuring no state leaks between tests.
func resetJety(t *testing.T) {
	t.Helper()
	defaultConfigManager = jety.NewConfigManager()
}
