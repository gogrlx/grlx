package local

import (
	"context"

	"github.com/gogrlx/grlx/ingredients/file"
	"github.com/gogrlx/grlx/types"
)

type LocalFile struct {
	ID          string
	Source      string
	Destination string
	Hash        string
	Props       map[string]interface{}
}

func (lf LocalFile) Download(context.Context) error {
	return nil
}

func (lf LocalFile) Properties() (map[string]interface{}, error) {
	return lf.Props, nil
}

func (lf LocalFile) Parse(id, source, destination, hash string, properties map[string]interface{}) (types.FileProvider, error) {
	return LocalFile{ID: id, Source: source, Destination: destination, Hash: hash, Props: properties}, nil
}

func (lf LocalFile) Protocols() []string {
	return []string{"file"}
}

func (lf LocalFile) Verify(context.Context) (bool, error) {
	return false, nil
}

func init() {
	file.RegisterProvider(LocalFile{})
}
