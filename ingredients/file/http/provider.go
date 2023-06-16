package http

import (
	"context"

	"github.com/gogrlx/grlx/ingredients/file"
	"github.com/gogrlx/grlx/types"
)

type HTTPFile struct {
	ID          string
	Source      string
	Destination string
	Hash        string
	Props       map[string]interface{}
}

func (hf HTTPFile) Download(context.Context) error {
	return nil
}

func (hf HTTPFile) Properties() (map[string]interface{}, error) {
	return hf.Props, nil
}

func (hf HTTPFile) Parse(id, source, destination, hash string, properties map[string]interface{}) (types.FileProvider, error) {
	return HTTPFile{ID: id, Source: source, Destination: destination, Hash: hash, Props: properties}, nil
}

func (hf HTTPFile) Protocols() []string {
	return []string{"http", "https"}
}

func (hf HTTPFile) Verify(context.Context) (bool, error) {
	return false, nil
}

func init() {
	file.RegisterProvider(HTTPFile{})
}
