package pki

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	log "github.com/taigrr/log-socket/log"

	"github.com/gogrlx/grlx/config"
	"github.com/gogrlx/grlx/types"
)

type PubKeyType int

const (
	SproutPubNKey PubKeyType = iota
	FarmerPubNKey
	CliPubNKey
)

var sproutMatcher *regexp.Regexp

func init() {
	sproutMatcher = regexp.MustCompile(`^[0-9a-z\.][-0-9_a-z\.]*$`)
}

func SetupPKIFarmer() {
	FarmerPKI := config.FarmerPKI
	_, err := os.Stat(FarmerPKI)
	if os.IsNotExist(err) {
		err = os.MkdirAll(FarmerPKI, os.ModePerm)
		if err != nil {
			// TODO check if no permissions to create, log, and then exit
			log.Panicf(err.Error())
		}
	}
	for _, acceptanceState := range []string{
		"unaccepted",
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
	SproutPKI := config.SproutPKI
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
		// TODO: work out what the other errors could be here
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

func createSproutID() string {
	id, err := os.Hostname()
	if err != nil {
		// TODO use another method of derivation instead of panicking
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
	newDest := filepath.Join(config.FarmerPKI + "sprouts/accepted/" + id)
	if fname == newDest {
		return types.ErrAlreadyAccepted
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
	newDest := filepath.Join(config.FarmerPKI + "sprouts/denied/" + id)
	fname, err := findNKey(id)
	if err != nil {
		return err
	}
	if fname == newDest {
		return types.ErrAlreadyDenied
	}
	return os.Rename(fname, newDest)
}

func UnacceptNKey(id string, nkey string) error {
	defer ReloadNKeys()
	newDest := filepath.Join(config.FarmerPKI + "sprouts/unaccepted/" + id)
	fname, err := findNKey(id)
	if nkey != "" && err == types.ErrSproutIDNotFound {
		file, errCreate := os.Create(newDest)
		if errCreate != nil {
			return errCreate
		}
		defer file.Close()
		_, errWrite := file.WriteString(nkey)
		return errWrite
	}
	if err != nil {
		return err
	}
	if fname == newDest {
		return types.ErrAlreadyUnaccepted
	}
	return os.Rename(fname, newDest)
}

func GetNKeysByType(set string) types.KeySet {
	keySet := types.KeySet{}
	keySet.Sprouts = []types.KeyManager{}
	switch set {
	case "unaccepted":
		fallthrough
	case "accepted":
		fallthrough
	case "denied":
		fallthrough
	case "rejected":
		// continue execution below default case
	default:
		return keySet
	}
	setPath := filepath.Join(config.FarmerPKI, "sprouts", set)
	filepath.WalkDir(setPath, func(path string, _ fs.DirEntry, _ error) error {
		_, id := filepath.Split(path)
		if setPath == path {
			return nil
		}
		keySet.Sprouts = append(keySet.Sprouts, types.KeyManager{SproutID: id})
		return nil
	})
	return keySet
}

func ListNKeysByType() types.KeysByType {
	var allKeys types.KeysByType
	allKeys.Accepted = GetNKeysByType("accepted")
	allKeys.Denied = GetNKeysByType("denied")
	allKeys.Rejected = GetNKeysByType("rejected")
	allKeys.Unaccepted = GetNKeysByType("unaccepted")
	return allKeys
}

func RejectNKey(id string, nkey string) error {
	defer ReloadNKeys()
	newDest := filepath.Join(config.FarmerPKI, "sprouts", "rejected", id)
	cleanDest := filepath.Clean(newDest)
	if newDest != cleanDest {
		return types.ErrSproutIDInvalid
	}
	fname, err := findNKey(id)
	if nkey != "" && err == types.ErrSproutIDNotFound {
		file, errCreate := os.Create(newDest)
		if errCreate != nil {
			return errCreate
		}
		defer file.Close()
		_, errWrite := file.WriteString(nkey)
		return errWrite
	}
	if err != nil {
		return err
	}
	if fname == newDest {
		return types.ErrAlreadyRejected
	}
	return os.Rename(fname, newDest)
}

func GetNKey(id string) (string, error) {
	FarmerPKI := config.FarmerPKI
	if !IsValidSproutID(id) {
		return "", types.ErrSproutIDInvalid
	}
	filename := ""
	filepath.WalkDir(FarmerPKI+"sprouts", func(path string, _ fs.DirEntry, _ error) error {
		switch path {
		case filepath.Join(FarmerPKI, "sprouts", "unaccepted", id):
			fallthrough
		case filepath.Join(FarmerPKI, "sprouts", "accepted", id):
			fallthrough
		case filepath.Join(FarmerPKI, "sprouts", "denied", id):
			fallthrough
		case filepath.Join(FarmerPKI, "sprouts", "rejected", id):
			filename = path
			return types.ErrSproutIDFound
		default:
		}
		return nil
	})
	if filename == "" {
		return "", types.ErrSproutIDNotFound
	}
	file, err := os.ReadFile(filename)
	return string(file), err
}

func findNKey(id string) (string, error) {
	FarmerPKI := config.FarmerPKI
	if !IsValidSproutID(id) {
		return "", types.ErrSproutIDInvalid
	}
	filename := ""
	filepath.WalkDir(FarmerPKI+"sprouts", func(path string, _ fs.DirEntry, _ error) error {
		switch path {
		case filepath.Join(FarmerPKI, "sprouts", "unaccepted", id):
			fallthrough
		case filepath.Join(FarmerPKI, "sprouts", "accepted", id):
			fallthrough
		case filepath.Join(FarmerPKI, "sprouts", "denied", id):
			fallthrough
		case filepath.Join(FarmerPKI, "sprouts", "rejected", id):
			filename = path
			return types.ErrSproutIDFound
		default:
		}
		return nil
	})
	if filename == "" {
		return "", types.ErrSproutIDNotFound
	}
	return filename, nil
}

func NKeyExists(id string, nkey string) (Registered bool, Matches bool) {
	FarmerPKI := config.FarmerPKI
	filename := ""
	filepath.WalkDir(filepath.Join(FarmerPKI, "sprouts"), func(path string, _ fs.DirEntry, _ error) error {
		switch path {
		case filepath.Join(FarmerPKI, "sprouts", "unaccepted", id):
			fallthrough
		case filepath.Join(FarmerPKI, "sprouts", "accepted", id):
			fallthrough
		case filepath.Join(FarmerPKI, "sprouts", "denied", id):
			fallthrough
		case filepath.Join(FarmerPKI, "sprouts", "rejected", id):
			filename = path
			return types.ErrSproutIDFound
		default:
		}
		return nil
	})
	if filename == "" {
		return false, false
	}
	file, err := os.ReadFile(filename)
	if err != nil {
		// TODO determine how we could get an error here
		log.Fatalf("Error reading in %s: %v", filename, err)
	}
	content := string(file)
	return true, content == nkey
}

func fetchRootCA(filename string) error {
	RootCA := filename
	_, err := os.Stat(RootCA)
	if err == nil {
		return err
	}
	if !os.IsNotExist(err) {
		return err
	}
	file, err := os.Create(RootCA)
	// TODO sort out this panic
	if err != nil {
		return err
	}
	defer file.Close()
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr, Timeout: time.Second * 10}
	r, err := client.Get(fmt.Sprintf("https://%s:%s/auth/cert/", config.FarmerInterface, config.FarmerAPIPort))
	if err != nil {
		os.Remove(RootCA)
		return err
	}
	_, err = io.Copy(file, r.Body)
	if err != nil {
		os.Remove(RootCA)
	}
	return err
}

func RootCACached(binary string) bool {
	var RootCA string
	switch binary {
	case "grlx":
		RootCA = config.GrlxRootCA
	case "sprout":
		RootCA = config.SproutRootCA
	}
	_, err := os.Stat(RootCA)
	return err == nil
}

var nkeyClient *http.Client

func LoadRootCA(binary string) error {
	nkeyClient = &http.Client{}
	var RootCA string
	switch binary {
	case "grlx":
		RootCA = config.GrlxRootCA
	case "sprout":
		RootCA = config.SproutRootCA
	}
	certPool := x509.NewCertPool()
	rootPEM, err := os.ReadFile(RootCA)
	if err != nil || rootPEM == nil {
		return err
	}
	ok := certPool.AppendCertsFromPEM(rootPEM)
	if !ok {
		log.Errorf("nats: failed to parse root certificate from %q", RootCA)
		return types.ErrCannotParseRootCA
	}
	var nkeyTransport http.RoundTripper = &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		TLSClientConfig: &tls.Config{
			RootCAs:    certPool,
			MinVersion: tls.VersionTLS12,
		},
	}
	nkeyClient.Transport = nkeyTransport
	nkeyClient.Timeout = time.Second * 10
	return nil
}

func PutNKey(id string) error {
	nkey, err := GetPubNKey(SproutPubNKey)
	if err != nil {
		return err
	}
	keySub := types.KeySubmission{NKey: nkey, SproutID: id}

	jw, _ := json.Marshal(keySub)
	url := config.FarmerURL + "/pki/putnkey"
	req, err := http.NewRequest(http.MethodPut, url, bytes.NewBuffer(jw))
	if err != nil {
		// handle error
		log.Fatal(err)
	}
	_, err = nkeyClient.Do(req)
	if err != nil {
		// TODO handle error
		return err
	}
	return nil
}

func GetPubNKey(keyType PubKeyType) (string, error) {
	var pubFile string
	switch keyType {
	case SproutPubNKey:
		pubFile = config.NKeySproutPubFile
	case FarmerPubNKey:
		pubFile = config.NKeyFarmerPubFile
		// case CliPubNKey:
		//	pubFile = config.NKeyGrlxPubFile
	}
	pubKeyBytes, err := os.ReadFile(pubFile)
	if err != nil {
		return "", err
	}
	return string(pubKeyBytes), nil
}

func GetSproutID() string {
	SproutID := config.SproutID
	if SproutID == "" {
		SproutID = createSproutID()
		config.SetSproutID(SproutID)
	}
	return SproutID
}
