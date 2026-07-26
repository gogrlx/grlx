package cook

import (
	"testing"

	"github.com/gogrlx/grlx/v2/internal/config"
)

func TestGetBasePathEnvOverride(t *testing.T) {
	old := config.RecipeDir
	defer func() { config.RecipeDir = old }()
	config.RecipeDir = "/srv/grlx/recipes/prod"

	t.Setenv(RecipeDirEnvVar, "/srv/grlx/recipes/feature-branch")
	if got := getBasePath(); got != "/srv/grlx/recipes/feature-branch" {
		t.Errorf("expected env override to win, got %q", got)
	}
}

func TestGetBasePathFallsBackToConfig(t *testing.T) {
	old := config.RecipeDir
	defer func() { config.RecipeDir = old }()
	config.RecipeDir = "/srv/grlx/recipes/prod"

	// Empty env var must fall back to the configured recipe dir.
	t.Setenv(RecipeDirEnvVar, "")
	if got := getBasePath(); got != "/srv/grlx/recipes/prod" {
		t.Errorf("expected config.RecipeDir fallback, got %q", got)
	}
}
