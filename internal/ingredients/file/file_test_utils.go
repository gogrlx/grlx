package file

import (
	"testing"

	"github.com/gogrlx/grlx/v2/types"
)

func compareResults(t *testing.T, result types.Result, expected types.Result) {
	if result.Succeeded != expected.Succeeded {
		t.Errorf("expected succeeded to be %v, got %v", expected.Succeeded, result.Succeeded)
	}
	if result.Failed != expected.Failed {
		t.Errorf("expected failed to be %v, got %v", expected.Failed, result.Failed)
	}
	if len(result.Notes) != len(expected.Notes) {
		t.Errorf("expected %v notes, got %v. Got %v", len(expected.Notes), len(result.Notes), result.Notes)
		return
	}
	for i, note := range result.Notes {
		if note.String() != expected.Notes[i].String() {
			t.Errorf("expected note `%v` to be `%s`, got `%s`", i, expected.Notes[i].String(), note.String())
		}
	}
}
