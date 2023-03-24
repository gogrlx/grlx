package file

import (
	"os"
	"time"

	"github.com/gogrlx/grlx/types"
)

// TODO allow selector to be more than an ID
func FAbsent(target types.KeyManager, file types.FilePath) (types.Result, error) {
	topic := "grlx.sprouts." + target.SproutID + ".file.absent"
	var results types.Result
	err := ec.Request(topic, file, &results, time.Second*15)
	return results, err
}

func SAbsent(file types.FilePath) (types.Result, error) {
	err := os.RemoveAll(file.Name)
	return types.Result{}, err
}

func SExists(file types.FilePath) (types.Result, error) {
	return types.Result{}, nil
}
