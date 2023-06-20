package http

import (
	"context"

	"github.com/gogrlx/grlx/ingredients/file"
	"github.com/gogrlx/grlx/ingredients/file/hashers"
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

func (lf HTTPFile) Verify(ctx context.Context) (bool, error) {
	hashType := ""
	if lf.Props["hashType"] == nil {
		hashType = hashers.GuessHashType(lf.Hash)
	} else if ht, ok := lf.Props["hashType"].(string); !ok {
		hashType = hashers.GuessHashType(lf.Hash)
	} else {
		hashType = ht
	}
	cf := hashers.CacheFile{
		ID:          lf.ID,
		Destination: lf.Destination,
		Hash:        lf.Hash,
		HashType:    hashType,
	}
	return cf.Verify(ctx)
}

func init() {
	file.RegisterProvider(HTTPFile{})
}
