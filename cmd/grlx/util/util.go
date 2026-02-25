package util

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	apitypes "github.com/gogrlx/grlx/v2/internal/api/types"
	"github.com/gogrlx/grlx/v2/internal/pki"
)

func UserChoice(first string, second string, options ...string) (string, error) {
	if len(first) == 0 || len(second) == 0 {
		panic(pki.ErrConfirmationLengthIsZero)
	}
	for _, option := range options {
		if len(option) == 0 {
			panic(pki.ErrConfirmationLengthIsZero)
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
	return "", apitypes.ErrInvalidUserInput
}

func UserChoiceWithDefault(def string, second string, options ...string) (string, error) {
	if len(def) == 0 || len(second) == 0 {
		panic(pki.ErrConfirmationLengthIsZero)
	}
	for _, option := range options {
		if len(option) == 0 {
			panic(pki.ErrConfirmationLengthIsZero)
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
	return "", apitypes.ErrInvalidUserInput
}

func UserConfirm(first string, second string) (bool, error) {
	if len(first) == 0 || len(second) == 0 {
		panic(pki.ErrConfirmationLengthIsZero)
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
		return false, apitypes.ErrInvalidUserInput
	}
}

func WriteJSON(i interface{}) {
	jw, _ := json.Marshal(i)
	fmt.Println(string(jw))
}

func WriteJSONErr(err error) {
	errWriter := apitypes.Inline{Success: false, Error: err}
	jw, _ := json.Marshal(errWriter)
	fmt.Println(string(jw))
}

func WriteOutput(i interface{}, mode string) {
	switch mode {
	case "json":
		WriteJSON(i)
	case "":
		fallthrough
	case "text":
		fmt.Println(i)
	}
}
