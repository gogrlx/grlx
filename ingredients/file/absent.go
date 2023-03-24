package file

import (
	"os"
	"time"

	"github.com/gogrlx/grlx/types"
)

// TODO allow selector to be more than an ID
func FAbsent(target types.KeyManager, file types.FileAbsent) (types.FileAbsent, error) {
	topic := "grlx.sprouts." + target.SproutID + ".file.absent"
	var results types.FileAbsent
	err := ec.Request(topic, file, &results, time.Second*15)
	return results, err
}

func SAbsent(file types.FileAbsent) (types.FileAbsent, error) {
	err := os.RemoveAll(file.Name)
	return file, err
}
