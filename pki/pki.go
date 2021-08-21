package pki

import (
	"crypto/tls"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gogrlx/grlx/config"
	. "github.com/gogrlx/grlx/config"
	. "github.com/gogrlx/grlx/types"
)

var sproutMatcher *regexp.Regexp

func init() {
	sproutMatcher = regexp.MustCompile(`^[0-9a-z\.][-0-9a-z\.]*$`)

}

func SetupPKIFarmer() {
	_, err := os.Stat(FarmerPKI)
	if err == nil {
		return
	}
	if os.IsNotExist(err) {
		err = os.MkdirAll(FarmerPKI, os.ModePerm)
		if err != nil {
			log.Panicf(err.Error())
		}
	} else {
		//TODO: work out what the other errors could be here
		log.Panicf(err.Error())
	}
	for _, acceptanceState := range []string{"unaccepted",
		"denied",
		"rejected",
		"accepted",
	} {
		_, err := os.Stat(FarmerPKI + "sprouts/" + acceptanceState)
		if err == nil {
			continue
		}
		if os.IsNotExist(err) {
			err = os.MkdirAll(FarmerPKI, os.ModePerm)
			if err != nil {
				log.Panicf(err.Error())
			}
		} else {
			//TODO: work out what the other errors could be here
			log.Panicf(err.Error())
		}

	}
}

func SetupPKISprout() {
	_, err := os.Stat(SproutPKI)
	if err == nil {
		return
	}
	if os.IsNotExist(err) {
		err = os.MkdirAll(SproutPKI, os.ModePerm)
		if err != nil {
			log.Panicf(err.Error())
		}
	} else {
		//TODO: work out what the other errors could be here
		log.Panicf(err.Error())
	}

}

// rules on sprout ids:
// must be unique
// if multiple sprouts claim the same id, the first one gets the id,
// following sprouts get id_n where n is their place in the queue
// sprout ids must be valid *nix hostnames: [0-9a-z\.][-0-9a-z\.]
// sprout ids cannot have underscores in their names, sprout daemons
// should automatically convert any found underscores to hyphens, unless
// the hostname starts with an underscore, in which case it is removed.
// maximum length is 253 characters
// trailing dots are not allowed

func CreateSproutID() string {
	id, err := os.Hostname()
	if err != nil {
		// TODO don't panic, use another method of derivation
		panic(err)
	}
	id = strings.ReplaceAll(id, "_", "-")
	id = strings.TrimPrefix(id, "-")
	return id
}

func IsValidSproutID(id string) bool {
	if strings.Contains(id, "_") {
		return false
	}
	if strings.HasPrefix(id, "-") {
		return false
	}
	if strings.HasSuffix(id, ".") {
		return false
	}
	if !sproutMatcher.MatchString(id) {
		return false
	}
	if len(id) < 254 {
		return true
	}
	return false
}
func AcceptNKey(id string) error {
	defer config.ReloadNKeys()
	newDest := FarmerPKI + "accepted/" + id
	fname, err := findNKey(id)
	if err != nil {
		return err
	}
	if fname == newDest {
		return ErrAlreadyAccepted
	}
	return os.Rename(fname, newDest)
}
func DeleteNKey(id string) error {
	defer config.ReloadNKeys()
	fname, err := findNKey(id)
	if err != nil {
		return err
	}
	return os.Remove(fname)
}
func DenyNKey(id string) error {
	defer config.ReloadNKeys()
	newDest := FarmerPKI + "denied/" + id
	fname, err := findNKey(id)
	if err != nil {
		return err
	}
	if fname == newDest {
		return ErrAlreadyDenied
	}
	return os.Rename(fname, newDest)

}
func UnacceptNKey(id string, nkey string) error {

	defer config.ReloadNKeys()
	newDest := FarmerPKI + "unaccepted/" + id
	fname, err := findNKey(id)
	if nkey != "" && err == ErrSproutIDNotFound {
		file, err := os.Create(newDest)
		if err != nil {
			return err
		}
		defer file.Close()
		_, err = file.WriteString(nkey)
		return err
	}
	if err != nil {
		return err
	}
	if fname == newDest {
		return ErrAlreadyUnaccepted
	}
	return os.Rename(fname, newDest)

}
func RejectNKey(id string, nkey string) error {
	defer config.ReloadNKeys()
	newDest := FarmerPKI + "rejected/" + id
	fname, err := findNKey(id)
	if nkey != "" && err == ErrSproutIDNotFound {
		file, err := os.Create(newDest)
		if err != nil {
			return err
		}
		defer file.Close()
		_, err = file.WriteString(nkey)
		return err
	}
	if err != nil {
		return err
	}
	if fname == newDest {
		return ErrAlreadyRejected
	}
	return os.Rename(fname, newDest)
}
func findNKey(id string) (string, error) {
	filename := ""
	filepath.WalkDir(FarmerPKI, func(path string, d fs.DirEntry, err error) error {
		switch path {
		case FarmerPKI + "unaccepted/" + id:
			fallthrough
		case FarmerPKI + "accepted/" + id:
			fallthrough
		case FarmerPKI + "denied/" + id:
			fallthrough
		case FarmerPKI + "rejected/" + id:
			filename = path
			return ErrSproutIDFound
		default:
		}
		return nil
	})
	if filename == "" {
		return "", ErrSproutIDNotFound
	}
	return filename, nil
}
func NKeyExists(id string, nkey string) (Registered bool, Matches bool) {
	filename := ""
	filepath.WalkDir(FarmerPKI, func(path string, d fs.DirEntry, err error) error {
		switch path {
		case FarmerPKI + "unaccepted/" + id:
			fallthrough
		case FarmerPKI + "accepted/" + id:
			fallthrough
		case FarmerPKI + "denied/" + id:
			fallthrough
		case FarmerPKI + "rejected/" + id:
			filename = path
			return ErrSproutIDFound
		default:
		}
		return nil
	})
	if filename == "" {
		return false, false
	}
	file, err := os.ReadFile(filename)
	if err != nil {
		//TODO determine how we could get an error here!
		log.Fatalf("Error reading in %s: %v", filename, err)
	}
	content := string(file)
	return true, content == nkey
}

func FetchRootCA() error {
	_, err := os.Stat(SproutRootCA)
	if err == nil {
		return err
	}
	if !os.IsNotExist(err) {
		return err
	}
	file, err := os.Create(SproutRootCA)
	//TODO: sort out this panic
	if err != nil {
		return err
	}
	defer file.Close()
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}
	r, err := client.Get("https://" + FarmerInterface + ":" + FarmerAPIPort + "/auth/cert/")
	if err != nil {
		os.Remove(SproutRootCA)
		return err
	}
	_, err = io.Copy(file, r.Body)
	if err != nil {
		os.Remove(SproutRootCA)
	}
	return err
}
