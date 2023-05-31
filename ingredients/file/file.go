package file

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/gogrlx/grlx/ingredients"
	"github.com/gogrlx/grlx/types"
	nats "github.com/nats-io/nats.go"
	"github.com/spf13/viper"
)

type File struct {
	ID     string
	Method string
}

// TODO error check, set id, properly parse
func (f File) Parse(id, method string, params map[string]interface{}) (types.RecipeCooker, error) {
	return New(id, method, params)
}

func New(id, method string, params map[string]interface{}) (File, error) {
	return File{ID: id, Method: method}, nil
}

func (f File) Test(ctx context.Context) (types.Result, error) {
	return types.Result{}, nil
}

func (f File) Apply(ctx context.Context) (types.Result, error) {
	switch f.Method {
	case "absent":
		fallthrough
	case "append":
		fallthrough
	case "contains":
		fallthrough
	case "content":
		fallthrough
	case "managed":
		fallthrough
	case "present":
		fallthrough
	case "symlink":
		fallthrough
	default:
		// TODO define error type
		return types.Result{Succeeded: false, Failed: true, Changed: false, Changes: nil}, fmt.Errorf("method %s undefined", f.Method)

	}
}

func (f File) PropertiesForMethod(method string) (map[string]string, error) {
	return nil, nil
}

func (f File) Methods() []string {
	return []string{
		"absent",
		"append",
		"contains",
		"content",
		"managed",
		"present",
		"symlink",
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
	ingredients.RegisterAllMethods(File{})
}
