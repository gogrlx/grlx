package pki

import (
	"strings"
	"testing"
)

func TestIsValidSproutID(t *testing.T) {
	testCases := []struct {
		id            string
		shouldSucceed bool
		testID        string
	}{
		{id: "test", shouldSucceed: true, testID: "test"},
		{id: "-test", shouldSucceed: false, testID: "leading hyphen"},
		{id: "te_st", shouldSucceed: true, testID: "embedded underscore"},
		{id: "grlxNode", shouldSucceed: false, testID: "capital letter"},
		{id: "t.est", shouldSucceed: true, testID: "embedded dot"},
		{id: strings.Repeat("a", 300), shouldSucceed: false, testID: "300 long string"},
		{id: strings.Repeat("a", 253), shouldSucceed: true, testID: "253 long string"},
		{id: "0132-465798qwertyuiopasdfghjklzxcv.bnm", shouldSucceed: true, testID: "keyboard smash"},
		{id: "te\nst", shouldSucceed: false, testID: "multiline"},
	}
	for _, tc := range testCases {
		t.Run(tc.testID, func(t *testing.T) {
			if IsValidSproutID(tc.id) != tc.shouldSucceed {
				t.Errorf("`%s`: expected %v but got %v", tc.id, tc.shouldSucceed, !tc.shouldSucceed)
			}
		})

	}
}
