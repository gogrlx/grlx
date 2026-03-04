package cook

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/gogrlx/grlx/v2/internal/config"
)

func TestMain(m *testing.M) {
	// Set RecipeDir to the test fixtures directory
	_, filename, _, _ := runtime.Caller(0)
	projectRoot := filepath.Dir(filepath.Dir(filepath.Dir(filename)))
	config.RecipeDir = filepath.Join(projectRoot, "testing", "recipes")
	os.Exit(m.Run())
}
