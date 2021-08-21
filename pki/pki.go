package pki

import (
	"log"
	"os"
	"regexp"
	"strings"

	. "github.com/gogrlx/grlx/config"
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
