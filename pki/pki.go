package pki

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"io"
	"io/fs"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	log "github.com/taigrr/log-socket/log"

	. "github.com/gogrlx/grlx/config"
	. "github.com/gogrlx/grlx/types"
)

var sproutMatcher *regexp.Regexp

func init() {
	sproutMatcher = regexp.MustCompile(`^[0-9a-z\.][-0-9_a-z\.]*$`)

}

func SetupPKIFarmer() {
	_, err := os.Stat(FarmerPKI)
	if os.IsNotExist(err) {
		err = os.MkdirAll(FarmerPKI, os.ModePerm)
		if err != nil {
			log.Panicf(err.Error())
		}
	}
	for _, acceptanceState := range []string{"unaccepted",
		"denied",
		"rejected",
		"accepted",
	} {
		stateFolder := filepath.Join(FarmerPKI + "sprouts/" + acceptanceState)
		_, err := os.Stat(stateFolder)
		if err == nil {
			continue
		}
		if os.IsNotExist(err) {
			err = os.MkdirAll(stateFolder, os.ModePerm)
			if err != nil {
				log.Panicf(err.Error())
			}
		} else {
			log.Fatal(err)
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
	if len(id) > 253 {
		return false
	}
	if strings.HasPrefix(id, "_") {
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

	return true
}
func AcceptNKey(id string) error {
	defer ReloadNKeys()
	fname, err := findNKey(id)
	if err != nil {
		return err
	}
	if len(strings.SplitN(id, "_", 2)) > 1 {
		DeleteNKey(strings.SplitN(id, "_", 2)[0])
	}
	id = strings.SplitN(id, "_", 2)[0]
	newDest := filepath.Join(FarmerPKI + "sprouts/accepted/" + id)
	if fname == newDest {
		return ErrAlreadyAccepted
	}
	return os.Rename(fname, newDest)
}
func DeleteNKey(id string) error {
	defer ReloadNKeys()
	fname, err := findNKey(id)
	if err != nil {
		return err
	}
	return os.Remove(fname)
}
func DenyNKey(id string) error {
	defer ReloadNKeys()
	newDest := filepath.Join(FarmerPKI + "sprouts/denied/" + id)
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
	defer ReloadNKeys()
	newDest := filepath.Join(FarmerPKI + "sprouts/unaccepted/" + id)
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
func GetNKeysByType(set string) KeySet {
	keySet := KeySet{}
	keySet.Sprouts = []KeyManager{}
	switch set {
	case "unaccepted":
		fallthrough
	case "accepted":
		fallthrough
	case "denied":
		fallthrough
	case "rejected":
	default:
		return keySet
	}
	setPath := filepath.Join(FarmerPKI + "sprouts/" + set)
	filepath.WalkDir(setPath, func(path string, d fs.DirEntry, err error) error {
		_, id := filepath.Split(path)
		keySet.Sprouts = append(keySet.Sprouts, KeyManager{SproutID: id})
		return nil
	})
	return keySet
}
func ListNKeysByType() KeysByType {
	var allKeys KeysByType
	allKeys.Accepted = GetNKeysByType("accepted")
	allKeys.Denied = GetNKeysByType("denied")
	allKeys.Rejected = GetNKeysByType("rejected")
	allKeys.Unaccepted = GetNKeysByType("unaccepted")
	return allKeys
}
func RejectNKey(id string, nkey string) error {
	defer ReloadNKeys()
	newDest := filepath.Join(FarmerPKI + "sprouts/rejected/" + id)
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
func GetNKey(id string) (string, error) {
	if !IsValidSproutID(id) {
		return "", ErrSproutIDInvalid
	}
	filename := ""
	filepath.WalkDir(FarmerPKI+"sprouts", func(path string, d fs.DirEntry, err error) error {
		switch path {
		case filepath.Join(FarmerPKI + "sprouts/unaccepted/" + id):
			fallthrough
		case filepath.Join(FarmerPKI + "sprouts/accepted/" + id):
			fallthrough
		case filepath.Join(FarmerPKI + "sprouts/denied/" + id):
			fallthrough
		case filepath.Join(FarmerPKI + "sprouts/rejected/" + id):
			filename = path
			return ErrSproutIDFound
		default:
		}
		return nil
	})
	if filename == "" {
		return "", ErrSproutIDNotFound
	}
	file, err := os.ReadFile(filename)
	return string(file), err
}
func findNKey(id string) (string, error) {
	if !IsValidSproutID(id) {
		return "", ErrSproutIDInvalid
	}
	filename := ""
	filepath.WalkDir(FarmerPKI+"sprouts", func(path string, d fs.DirEntry, err error) error {
		switch path {
		case filepath.Join(FarmerPKI + "sprouts/unaccepted/" + id):
			fallthrough
		case filepath.Join(FarmerPKI + "sprouts/accepted/" + id):
			fallthrough
		case filepath.Join(FarmerPKI + "sprouts/denied/" + id):
			fallthrough
		case filepath.Join(FarmerPKI + "sprouts/rejected/" + id):
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
	filepath.WalkDir(FarmerPKI+"sprouts/", func(path string, d fs.DirEntry, err error) error {
		switch path {
		case filepath.Join(FarmerPKI + "sprouts/unaccepted/" + id):
			fallthrough
		case filepath.Join(FarmerPKI + "sprouts/accepted/" + id):
			fallthrough
		case filepath.Join(FarmerPKI + "sprouts/denied/" + id):
			fallthrough
		case filepath.Join(FarmerPKI + "sprouts/rejected/" + id):
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

func fetchRootCA() error {
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
	client := &http.Client{Transport: tr, Timeout: time.Second * 10}
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
func RootCACached() bool {
	_, err := os.Stat(SproutRootCA)
	if err == nil {
		return true
	}
	return false
}
func LoadRootCA() error {
	if err := fetchRootCA(); err != nil {
		return err
	}
	certPool := x509.NewCertPool()
	rootPEM, err := ioutil.ReadFile(SproutRootCA)
	if err != nil || rootPEM == nil {
		return err
	}
	ok := certPool.AppendCertsFromPEM(rootPEM)
	if !ok {
		log.Errorf("nats: failed to parse root certificate from %q", SproutRootCA)
		return ErrCannotParseRootCA
	}
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{
		RootCAs:    certPool,
		MinVersion: tls.VersionTLS12,
	}
	http.DefaultClient.Timeout = time.Second * 10
	return nil
}

//TODO handle return body
func PutNKey(id string) error {
	nkey, err := GetPubNKey(false)
	if err != nil {
		return err
	}
	keySub := KeySubmission{NKey: nkey, SproutID: id}

	jw, _ := json.Marshal(keySub)
	url := FarmerURL + "/pki/putnkey"
	req, err := http.NewRequest(http.MethodPut, url, bytes.NewBuffer(jw))
	if err != nil {
		// handle error
		log.Fatal(err)
	}
	_, err = http.DefaultClient.Do(req)
	if err != nil {
		// handle error
		return err
	}
	return nil
}

func GetPubNKey(isFarmer bool) (string, error) {
	pubFile := NKeySproutPubFile
	if isFarmer {
		pubFile = NKeyFarmerPubFile
	}
	pubKeyBytes, err := os.ReadFile(pubFile)
	if err != nil {
		return "", err
	}
	return string(pubKeyBytes), nil
}

func GetSproutID() string {
	if SproutID == "" {
		return CreateSproutID()
	}
	return SproutID
}
