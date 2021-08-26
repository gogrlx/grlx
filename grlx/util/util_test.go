package util

import (
	"testing"
)

func Test_listIntersection(t *testing.T) {
	testCases := []struct {
		id  string
		a   []string
		b   []string
		res []string
	}{{a: []string{"1", "2", "3", "4"},
		b:   []string{"1", "2", "4", "4", "5"},
		res: []string{"1", "2", "4"},
		id:  "deduplicated overlap"},
		{a: []string{}, b: []string{}, res: []string{}, id: "all empty"},
		{a: []string{"a"}, b: []string{}, res: []string{}, id: "b empty"},
		{a: []string{}, b: []string{"b"}, res: []string{}, id: "a empty"},
	}
	for _, tc := range testCases {
		t.Run(tc.id, func(t *testing.T) {
			overlap := listIntersection(&tc.a, &tc.b)
			if len(overlap) != len(tc.res) {
				t.Errorf("Expected and actual overlaps have different lengths.")

			}
			for i, x := range overlap {
				if x != tc.res[i] {
					t.Errorf("found %s != %s ", x, tc.res[i])
				}
			}
		})
	}
}
func Test_targetRegex(t *testing.T) {
	testCases := []struct {
		id  string
		a   []string
		t   string
		res []string
	}{{a: []string{"b", "bc", "abcd", "bcde"}, t: "b", res: []string{"b"}, id: "lazy match"},
		{a: []string{"b", "bc", "abcd", "bcde"}, t: "^b$", res: []string{"b"}, id: "lazy match with ^$"},
		{a: []string{"b", "bc", "abcd", "bcde"}, t: ".*b", res: []string{"b"}, id: "*b"},
		{a: []string{"b", "bc", "abcd", "bcde"}, t: ".*", res: []string{"b", "bc", "abcd", "bcde"}, id: "match all"},
		{a: []string{"b", "bc", "abcd", "bcde"}, t: "", res: []string{}, id: "empty string"},
		{a: []string{"b", "bc", "abcd", "bcde"}, t: "bc+", res: []string{"bc"}, id: "match repeating c's"},
		{a: []string{"b", "bc", "abcd", "bcde"}, t: "bc.*", res: []string{"bc", "bcde"}, id: "match with .*"},
	}
	for _, tc := range testCases {
		t.Run(tc.id, func(t *testing.T) {
			matches, _ := targetRegex(tc.t, &tc.a)
			if len(matches) != len(tc.res) {
				t.Errorf("Expected and actual matches have different lengths.")
			}
			for i, x := range matches {
				if x != tc.res[i] {
					t.Errorf("found %s != %s ", x, tc.res[i])
				}
			}
		})
	}
}
