package client

import (
	"regexp"
	"strings"
)

// Target matching behavior is as follows:
// If the target string contains commas, it's considered to be a literal list
// Otherwise, the target string will be treated as a regular expression with
// an implicit '^' prepended and '$' appended.
// To avoid RegExp matching, use a trailing or preceding comma in the target.
func ResolveTargets(target string) ([]string, error) {
	validKeys, err := ListKeys()
	if err != nil {
		return []string{}, err
	}
	accepted := []string{}
	for _, km := range validKeys.Accepted.Sprouts {
		accepted = append(accepted, km.SproutID)
	}

	if strings.ContainsRune(target, ',') {
		targetList := strings.Split(target, ",")
		return listIntersection(&targetList, &accepted), nil
	}

	return targetRegex(target, &accepted)
}

func targetRegex(target string, accepted *[]string) ([]string, error) {
	if !strings.HasPrefix(target, "^") {
		target = "^" + target
	}
	if !strings.HasSuffix(target, "$") {
		target = target + "$"
	}
	re, err := regexp.Compile(target)
	if err != nil {
		return []string{}, err
	}
	matchedTargets := []string{}
	for _, x := range *accepted {
		if re.MatchString(x) {
			matchedTargets = append(matchedTargets, x)
		}
	}
	return matchedTargets, nil
}

func listIntersection(a *[]string, b *[]string) []string {
	hash := make(map[string]bool)
	overlap := []string{}
	for _, id := range *a {
		hash[id] = true
	}
	for _, id := range *b {
		if hash[id] {
			overlap = append(overlap, id)
			hash[id] = false
		}
	}
	return overlap
}
