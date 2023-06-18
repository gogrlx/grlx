package user

import "os/user"

func UserExists(username string) (bool, error) {
	u, err := user.Lookup(username)
	if err != nil {
		return false, err
	}
	return u != nil, nil
}
