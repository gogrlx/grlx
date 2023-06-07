package util

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"

	gpki "github.com/gogrlx/grlx/pkg/grlx/pki"
	"github.com/gogrlx/grlx/types"
)

func UserChoice(first string, second string, options ...string) (string, error) {
	if len(first) == 0 || len(second) == 0 {
		panic(types.ErrConfirmationLengthIsZero)
	}
	for _, option := range options {
		if len(option) == 0 {
			panic(types.ErrConfirmationLengthIsZero)
		}
	}
	fmt.Printf("%s/%s", first, second)
	for _, option := range options {
		fmt.Printf("/%s", option)
	}
	fmt.Print(":")
	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		panic(err)
	}
	input = strings.TrimSuffix(input, "\n")
	switch strings.ToLower(input) {
	case strings.ToLower(first):
		return first, nil
	case strings.ToLower(second):
		return second, nil
	}
	for _, option := range options {
		if strings.EqualFold(option, input) {
			return option, nil
		}
	}
	return "", types.ErrInvalidUserInput
}

func UserChoiceWithDefault(def string, second string, options ...string) (string, error) {
	if len(def) == 0 || len(second) == 0 {
		panic(types.ErrConfirmationLengthIsZero)
	}
	for _, option := range options {
		if len(option) == 0 {
			panic(types.ErrConfirmationLengthIsZero)
		}
	}
	fmt.Printf("%s/%s", strings.ToTitle(def), strings.ToLower(second))
	for _, option := range options {
		fmt.Printf("/%s", strings.ToLower(option))
	}
	fmt.Print(":")
	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		panic(err)
	}
	input = strings.TrimSuffix(input, "\n")
	switch strings.ToLower(input) {
	case "":
		fallthrough
	case strings.ToLower(def):
		return def, nil
	case strings.ToLower(second):
		return second, nil
	}
	for _, option := range options {
		if strings.EqualFold(option, input) {
			return option, nil
		}
	}
	return "", types.ErrInvalidUserInput
}

func UserConfirm(first string, second string) (bool, error) {
	if len(first) == 0 || len(second) == 0 {
		panic(types.ErrConfirmationLengthIsZero)
	}
	fmt.Printf("%s/%s", strings.ToLower(first), strings.ToLower(second))
	fmt.Print(":")
	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		panic(err)
	}
	input = strings.TrimSuffix(input, "\n")
	switch strings.ToLower(input) {
	case strings.ToLower(first):
		return true, nil
	case strings.ToLower(second):
		return true, nil
	default:
		return false, nil
	}
}

func UserConfirmWithDefault(def bool) (bool, error) {
	if def {
		fmt.Print("Y/n:")
	} else {
		fmt.Print("y/N:")
	}
	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		panic(err)
	}
	input = strings.TrimSuffix(input, "\n")
	switch strings.ToLower(input) {
	case "":
		return def, nil
	case "y":
		return true, nil
	case "n":
		return false, nil
	default:
		return false, types.ErrInvalidUserInput
	}
}

// Target matching behavior is as follows:
// If the target string contains commas, it's considered to be a literal list
// Otherwise, the target string will be treated as a regular expression with
// an implicit '^' prepended and '$' appended.
// To avoid RegExp matching, use a trailing or preceding comma in the target.
func ResolveTargets(target string) ([]string, error) {
	validKeys, err := gpki.ListKeys()
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

func WriteYAML(i interface{}) {
	jy, _ := yaml.Marshal(i)
	fmt.Println(string(jy))
}

func WriteJSON(i interface{}) {
	jw, _ := json.Marshal(i)
	fmt.Println(string(jw))
}

func WriteJSONErr(err error) {
	errWriter := types.Inline{Success: false, Error: err}
	jw, _ := json.Marshal(errWriter)
	fmt.Println(string(jw))
}

func WriteYAMLErr(err error) {
	errWriter := types.Inline{Success: false, Error: err}
	yw, _ := yaml.Marshal(errWriter)
	fmt.Println(string(yw))
}

func WriteOutput(i interface{}, mode string) {
	switch mode {
	case "json":
		WriteJSON(i)
	case "yaml":
		WriteYAML(i)
	case "":
		fallthrough
	case "text":
		fmt.Println(i)
	}
}
