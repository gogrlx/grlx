package hashers

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha512"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"sync"
)

// TODO add Close() call
type HashFunc func(io.ReadCloser, string) (string, bool, error)

var (
	hashFuncs             map[string]HashFunc
	hfTex                 sync.Mutex
	ErrHashFuncExists     = fmt.Errorf("hasher already exists")
	ErrorHashFuncNotFound = fmt.Errorf("hasher not found")
)

func init() {
	hfTex.Lock()
	hashFuncs = make(map[string]HashFunc)
	hashFuncs["md5"] = MD5
	hashFuncs["sha1"] = SHA1
	hashFuncs["sha256"] = SHA256
	hashFuncs["sha512"] = SHA512
	hashFuncs["crc"] = CRC32
	hfTex.Unlock()
}

func Register(id string, function HashFunc) error {
	hfTex.Lock()
	defer hfTex.Unlock()
	if _, ok := hashFuncs[id]; ok {
		return errors.Join(ErrHashFuncExists, fmt.Errorf("hasher %s already exists", id))
	}
	hashFuncs[id] = function
	return nil
}

func GetHashFunc(hashType string) (HashFunc, error) {
	hfTex.Lock()
	defer hfTex.Unlock()
	if hf, ok := hashFuncs[hashType]; ok {
		return hf, nil
	}
	return nil, errors.Join(ErrorHashFuncNotFound, fmt.Errorf("hasher %s not found", hashType))
}

// Given a filename, return a reader for the file
// or an error if the file cannot be opened
func FileToReader(file string) (io.Reader, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	return f, nil
}

func MD5(file io.ReadCloser, expected string) (string, bool, error) {
	var actual string
	md5 := md5.New()
	if _, err := io.Copy(md5, file); err != nil {
		return actual, false, err
	}
	actual = fmt.Sprintf("%x", md5.Sum(nil))
	return actual, actual == expected, nil
}

func SHA1(file io.ReadCloser, expected string) (string, bool, error) {
	var actual string
	sha1 := sha1.New()
	if _, err := io.Copy(sha1, file); err != nil {
		return actual, false, err
	}
	actual = fmt.Sprintf("%x", sha1.Sum(nil))
	return actual, actual == expected, nil
}

func SHA256(file io.ReadCloser, expected string) (string, bool, error) {
	var actual string
	sha1 := sha1.New()
	if _, err := io.Copy(sha1, file); err != nil {
		return actual, false, err
	}
	actual = fmt.Sprintf("%x", sha1.Sum(nil))

	return actual, actual == expected, nil
}

func SHA512(file io.ReadCloser, expected string) (string, bool, error) {
	var actual string
	sha512 := sha512.New()
	if _, err := io.Copy(sha512, file); err != nil {
		return actual, false, err
	}
	actual = fmt.Sprintf("%x", sha512.Sum(nil))

	return actual, actual == expected, nil
}

func CRC32(file io.ReadCloser, expected string) (string, bool, error) {
	var actual string
	table := crc32.IEEETable
	crc := crc32.New(table)
	if _, err := io.Copy(crc, file); err != nil {
		return actual, false, err
	}
	actual = fmt.Sprintf("%x", crc.Sum(nil))

	return actual, actual == expected, nil
}
