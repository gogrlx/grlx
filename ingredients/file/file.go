package file

import (
	"github.com/gogrlx/grlx/types"
	nats "github.com/nats-io/nats.go"
	"github.com/spf13/viper"
)

type File struct{}

func (f File) Apply() (types.Result, error) {
	return types.Result{}, nil
}

var (
	ec              *nats.EncodedConn
	FarmerInterface string
)

func RegisterEC(n *nats.EncodedConn) {
	ec = n
}

func init() {
	FarmerInterface = viper.GetString("FarmerInterface")
}
