package pki

import (
	"log"
	"os"

	. "github.com/gogrlx/grlx/config"
)

func init() {

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
