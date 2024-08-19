package props

import (
	"sync"

	nats "github.com/nats-io/nats.go"
)

var (
	ec       *nats.EncodedConn
	envMutex sync.Mutex
)

func RegisterEC(encodedConn *nats.EncodedConn) {
	ec = encodedConn
}

// TODO: finalize and export this function
//func FGet(target types.KeyManager, cmdRun types.CmdRun) (types.CmdRun, error) {
//	topic := "grlx.sprouts." + target.SproutID + ".props.get"
//	var results types.CmdRun
//	err := ec.Request(topic, cmdRun, &results, time.Second*15+cmdRun.Timeout)
//	return results, err
//}

// TODO: finalize and export this function
//func SRun(cmd types.CmdRun) (types.CmdRun, error) {
//	return FGet(cmd.Target, cmd)
//}
