package hashers

import "io"

type HashFunc func(io.Reader, string) (string, bool, error)

var HashFuncs map[string]HashFunc

func init() {
	HashFuncs = make(map[string]HashFunc)
	HashFuncs["md5"] = MD5
	HashFuncs["sha1"] = SHA1
	HashFuncs["sha256"] = SHA256
	HashFuncs["sha512"] = SHA512
	HashFuncs["crc"] = CRC
}

func MD5(file io.Reader, expected string) (string, bool, error) {
	var actual string
	var err error
	return actual, actual == expected, err
}

func SHA1(file io.Reader, expected string) (string, bool, error) {
	var actual string
	var err error
	return actual, actual == expected, err
}

func SHA256(file io.Reader, expected string) (string, bool, error) {
	var actual string
	var err error
	return actual, actual == expected, err
}

func SHA512(file io.Reader, expected string) (string, bool, error) {
	var actual string
	var err error
	return actual, actual == expected, err
}

func CRC(file io.Reader, expected string) (string, bool, error) {
	var actual string
	var err error
	return actual, actual == expected, err
}
