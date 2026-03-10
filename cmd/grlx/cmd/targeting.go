package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/gogrlx/grlx/v2/internal/api/client"
)

// addTargetFlags adds -T and -C flags to a command.
// They are mutually exclusive: one must be provided.
func addTargetFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().StringVarP(&sproutTarget, "target", "T", "", "sprout target (comma-separated list or regex)")
	cmd.PersistentFlags().StringVarP(&cohortTarget, "cohort", "C", "", "cohort name to resolve as target")
}

// resolveEffectiveTarget returns the sprout target string from either -T or -C.
// If -C is set, it resolves the cohort via the farmer API and returns a
// comma-separated list of sprout IDs. If neither or both are set, it errors.
func resolveEffectiveTarget() (string, error) {
	hasTarget := sproutTarget != ""
	hasCohort := cohortTarget != ""

	if hasTarget && hasCohort {
		return "", fmt.Errorf("cannot use both --target (-T) and --cohort (-C)")
	}
	if !hasTarget && !hasCohort {
		return "", fmt.Errorf("either --target (-T) or --cohort (-C) is required")
	}

	if hasTarget {
		return sproutTarget, nil
	}

	// Resolve cohort via farmer
	sprouts, err := client.ResolveCohort(cohortTarget)
	if err != nil {
		return "", fmt.Errorf("cohort %q: %w", cohortTarget, err)
	}

	fmt.Printf("Cohort %q resolved to %d sprout(s): %s\n", cohortTarget, len(sprouts), strings.Join(sprouts, ", "))
	return strings.Join(sprouts, ","), nil
}
