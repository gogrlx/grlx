package http

import (
	"context"
	"fmt"
	"io"
	httpc "net/http"
	"os"

	// "github.com/gogrlx/grlx/v2/internal/ingredients/file"
	"github.com/gogrlx/grlx/v2/internal/ingredients/file/hashers"
	"github.com/gogrlx/grlx/v2/types"
)

type HTTPFile struct {
	ID          string
	Source      string
	Destination string
	Hash        string
	Props       map[string]interface{}
}

func (hf HTTPFile) Download(ctx context.Context) error {
	dest, err := os.Create(hf.Destination)
	if err != nil {
		return err
	}
	defer dest.Close()

	method := httpc.MethodGet
	if hf.Props["method"] != nil {
		if m, okM := hf.Props["method"].(string); okM {
			method = m
		}
	}
	// TODO add headers, other body settings, etc here
	req, err := httpc.NewRequestWithContext(ctx, method, hf.Source, nil)
	if err != nil {
		return err
	}
	res, err := httpc.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	expectedCode := httpc.StatusOK
	if hf.Props["expectedCode"] != nil {
		if ec, okEC := hf.Props["expectedCode"].(int); okEC {
			expectedCode = ec
		}
	}
	if res.StatusCode != expectedCode {
		// TODO standardize this error message
		return fmt.Errorf("unexpected HTTP status code %d", res.StatusCode)
	}
	_, err = io.Copy(dest, res.Body)
	if err != nil {
		return err
	}
	return nil
}

func (hf HTTPFile) Properties() (map[string]interface{}, error) {
	return hf.Props, nil
}

func (hf HTTPFile) Parse(id, source, destination, hash string, properties map[string]interface{}) (types.FileProvider, error) {
	if properties == nil {
		properties = make(map[string]interface{})
	}
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

// func init() {
// 	file.RegisterProvider(HTTPFile{})
// }
