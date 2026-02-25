package s3

import (
	"context"

	"github.com/gogrlx/grlx/v2/internal/ingredients/file"
)

type S3File struct {
	ID          string
	Source      string
	Destination string
	Hash        string
	Props       map[string]interface{}
}

func (sf S3File) Download(context.Context) error {
	return nil
}

func (sf S3File) Properties() (map[string]interface{}, error) {
	return sf.Props, nil
}

func (sf S3File) Parse(id, source, destination, hash string, properties map[string]interface{}) (file.FileProvider, error) {
	if properties == nil {
		properties = make(map[string]interface{})
	}
	return S3File{ID: id, Source: source, Destination: destination, Hash: hash, Props: properties}, nil
}

func (sf S3File) Protocols() []string {
	return []string{"file"}
}

func (sf S3File) Verify(context.Context) (bool, error) {
	return false, nil
}
