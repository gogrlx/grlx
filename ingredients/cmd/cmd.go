package cmd

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
func New(id, method string, params map[string]interface{}) Cmd {
	return Cmd{ID: id, Method: method}
}

func (c Cmd) Test(ctx context.Context) (types.Result, error) {
	return types.Result{}, nil
}

func (c Cmd) Apply(ctx context.Context) (types.Result, error) {
	switch c.Method {
	case "cmd.run":
		fallthrough
	default:
		// TODO define error type
		return types.Result{Succeeded: false, Failed: true, Changed: false, Changes: nil}, fmt.Errorf("method %s undefined", c.Method)

	}
}

func (c Cmd) Methods() []string{
	return []string{"cmd.run", 
		}
}

func (c Cmd) Properties() (map[string]interface{}, error) {
	m := map[string]interface{}{}
	b, err := json.Marshal(c)
	if err != nil {
		return m, err
	}
	err = json.Unmarshal(b, &m)
	return m, err
}
