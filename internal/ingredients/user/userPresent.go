package user

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strings"

	"github.com/gogrlx/grlx/v2/internal/cook"
)

// execCommandContext is a test-overridable factory for exec.Cmd.
var execCommandContext = exec.CommandContext

// lookupUser is a test-overridable wrapper around user.Lookup.
var lookupUser = user.Lookup

// validHashPrefixes lists accepted crypt(3) hash algorithm prefixes.
var validHashPrefixes = []string{
	"$1$",  // MD5
	"$5$",  // SHA-256
	"$6$",  // SHA-512
	"$y$",  // yescrypt
	"$2b$", // bcrypt
	"$2a$", // bcrypt (older)
}

// isValidPasswordHash checks that a password hash string starts with a
// recognised crypt(3) prefix.
func isValidPasswordHash(hash string) bool {
	for _, prefix := range validHashPrefixes {
		if strings.HasPrefix(hash, prefix) {
			return true
		}
	}
	return false
}

// shadowPasswordHash reads /etc/shadow and returns the current password hash
// for the given username.  Returns an empty string when the user is not found
// or the file is unreadable.
var shadowFilePath = "/etc/shadow"

func shadowPasswordHash(username string) (string, error) {
	f, err := os.Open(shadowFilePath)
	if err != nil {
		return "", fmt.Errorf("cannot read shadow file: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	prefix := username + ":"
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		fields := strings.SplitN(line, ":", 3)
		if len(fields) < 2 {
			return "", nil
		}
		return fields[1], nil
	}
	return "", nil
}

func (u User) present(ctx context.Context, test bool) (cook.Result, error) {
	var result cook.Result

	userName, ok := u.params["name"].(string)
	if !ok || userName == "" {
		result.Failed = true
		return result, errors.New("invalid user; name must be a non-empty string")
	}

	uid := stringParam(u.params, "uid")
	gid := stringParam(u.params, "gid")
	shell := stringParam(u.params, "shell")
	home := stringParam(u.params, "home")
	comment := stringParam(u.params, "comment")
	passwordHash := stringParam(u.params, "password_hash")
	groups := stringSliceParam(u.params, "groups")
	createHome := boolParam(u.params, "createhome", true)
	system := boolParam(u.params, "system", false)

	// Validate password hash format if provided.
	if passwordHash != "" && !isValidPasswordHash(passwordHash) {
		result.Failed = true
		return result, fmt.Errorf("invalid password_hash: must start with a recognised crypt prefix (%s)",
			strings.Join(validHashPrefixes, ", "))
	}

	existing, err := lookupUser(userName)
	if err != nil {
		// User does not exist — useradd
		args := buildUseraddArgs(userName, uid, gid, shell, home, comment, passwordHash, groups, createHome, system)
		cmd := execCommandContext(ctx, "useradd", args...)
		if test {
			result.Succeeded = true
			result.Changed = true
			result.Notes = append(result.Notes,
				cook.SimpleNote(fmt.Sprintf("would create user with command: %s", cmd.String())))
			return result, nil
		}
		if err := cmd.Run(); err != nil {
			result.Failed = true
			result.Notes = append(result.Notes,
				cook.SimpleNote(fmt.Sprintf("failed to create user: %s", err.Error())))
			return result, err
		}
		result.Succeeded = true
		result.Changed = true
		result.Notes = append(result.Notes,
			cook.SimpleNote(fmt.Sprintf("created user with command: %s", cmd.String())))
		return result, nil
	}

	// User exists — check if modifications are needed
	args := buildUsermodArgs(userName, uid, gid, shell, home, comment, passwordHash, groups, existing)
	if len(args) == 0 {
		// No changes needed
		result.Succeeded = true
		result.Notes = append(result.Notes,
			cook.SimpleNote(fmt.Sprintf("user %s is already in the desired state", userName)))
		return result, nil
	}

	cmd := execCommandContext(ctx, "usermod", args...)
	if test {
		result.Succeeded = true
		result.Changed = true
		result.Notes = append(result.Notes,
			cook.SimpleNote(fmt.Sprintf("would modify user with command: %s", cmd.String())))
		return result, nil
	}
	if err := cmd.Run(); err != nil {
		result.Failed = true
		result.Notes = append(result.Notes,
			cook.SimpleNote(fmt.Sprintf("failed to modify user: %s", err.Error())))
		return result, err
	}
	result.Succeeded = true
	result.Changed = true
	result.Notes = append(result.Notes,
		cook.SimpleNote(fmt.Sprintf("modified user with command: %s", cmd.String())))
	return result, nil
}

func buildUseraddArgs(name, uid, gid, shell, home, comment, passwordHash string, groups []string, createHome, system bool) []string {
	var args []string
	if uid != "" {
		args = append(args, "-u", uid)
	}
	if gid != "" {
		args = append(args, "-g", gid)
	}
	if shell != "" {
		args = append(args, "-s", shell)
	}
	if home != "" {
		args = append(args, "-d", home)
	}
	if comment != "" {
		args = append(args, "-c", comment)
	}
	if passwordHash != "" {
		args = append(args, "-p", passwordHash)
	}
	if len(groups) > 0 {
		args = append(args, "-G", strings.Join(groups, ","))
	}
	if createHome {
		args = append(args, "-m")
	}
	if system {
		args = append(args, "-r")
	}
	args = append(args, name)
	return args
}

func buildUsermodArgs(name, uid, gid, shell, home, comment, passwordHash string, groups []string, existing *user.User) []string {
	var args []string
	if uid != "" && uid != existing.Uid {
		args = append(args, "-u", uid)
	}
	if gid != "" && gid != existing.Gid {
		args = append(args, "-g", gid)
	}
	if shell != "" {
		// user.User doesn't expose shell, so always set if specified
		args = append(args, "-s", shell)
	}
	if home != "" && home != existing.HomeDir {
		args = append(args, "-d", home)
	}
	if comment != "" {
		// user.User.Name is the GECOS field
		if comment != existing.Name {
			args = append(args, "-c", comment)
		}
	}
	if passwordHash != "" {
		// Check if the hash differs from the current shadow entry.
		currentHash, _ := shadowPasswordHash(name)
		if currentHash != passwordHash {
			args = append(args, "-p", passwordHash)
		}
	}
	if len(groups) > 0 {
		args = append(args, "-G", strings.Join(groups, ","))
	}
	if len(args) == 0 {
		return nil
	}
	args = append(args, name)
	return args
}

// stringParam extracts a string parameter from the params map.
func stringParam(params map[string]interface{}, key string) string {
	v, ok := params[key]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

// stringSliceParam extracts a []string parameter, handling both []string
// and []interface{} (which is what JSON unmarshalling produces).
func stringSliceParam(params map[string]interface{}, key string) []string {
	v, ok := params[key]
	if !ok {
		return nil
	}
	switch vt := v.(type) {
	case []string:
		return vt
	case []interface{}:
		var out []string
		for _, item := range vt {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

// boolParam extracts a bool parameter with a default value.
func boolParam(params map[string]interface{}, key string, defaultVal bool) bool {
	v, ok := params[key]
	if !ok {
		return defaultVal
	}
	b, ok := v.(bool)
	if !ok {
		return defaultVal
	}
	return b
}
