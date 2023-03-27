package file

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/gogrlx/grlx/types"
	nats "github.com/nats-io/nats.go"
	"github.com/spf13/viper"
)

type File struct {
	ID     string
	Method string
}

func New(id, method string, params map[string]interface{}) File {
	return File{ID: id, Method: method}
}

func (f File) Test(ctx context.Context) (types.Result, error) {
	return types.Result{}, nil
}

func (f File) Apply(ctx context.Context) (types.Result, error) {
	switch f.Method {
	case "absent":
		fallthrough
	case "managed":
		fallthrough
	case "append":
		return types.Result{Succeeded: true}, nil
	default:
		// TODO define error type
		return types.Result{Succeeded: false, Failed: true, Changed: false, Changes: nil}, fmt.Errorf("method %s undefined", f.Method)

	}
}

func (f File) Properties() (map[string]interface{}, error) {
	m := map[string]interface{}{}
	b, err := json.Marshal(f)
	if err != nil {
		return m, err
	}
	err = json.Unmarshal(b, &m)
	return m, err
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
