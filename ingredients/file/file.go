package file

import (
	nats "github.com/nats-io/nats.go"
	"github.com/spf13/viper"
)

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
