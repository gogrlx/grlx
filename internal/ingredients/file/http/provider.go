package http

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	httpc "net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/gogrlx/grlx/v2/internal/ingredients/file"
	"github.com/gogrlx/grlx/v2/internal/ingredients/file/hashers"
)

type HTTPFile struct {
	ID          string
	Source      string
	Destination string
	Hash        string
	Props       map[string]interface{}
}

const (
	downloadTempPattern       = ".grlx-http-download-*"
	expectedCodeProperty      = "expectedCode"
	defaultExpectedStatusCode = httpc.StatusOK
	maxExpectedStatusCode     = 999
)

// Compile-time interface check.
var _ file.FileProvider = HTTPFile{}

func (hf HTTPFile) Download(ctx context.Context) error {
	method := httpc.MethodGet
	if hf.Props["method"] != nil {
		if m, okM := hf.Props["method"].(string); okM {
			method = m
		}
	}
	req, err := httpc.NewRequestWithContext(ctx, method, hf.Source, nil)
	if err != nil {
		return err
	}
	if err := applyRequestHeaders(req, hf.Props["headers"]); err != nil {
		return err
	}
	res, err := httpc.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	expectedCode := expectedStatusCode(hf.Props)
	if res.StatusCode != expectedCode {
		// TODO standardize this error message
		return fmt.Errorf("unexpected HTTP status code %d", res.StatusCode)
	}
	destinationDir := filepath.Dir(hf.Destination)
	stagedFile, err := os.CreateTemp(destinationDir, downloadTempPattern)
	if err != nil {
		return err
	}
	stagedPath := stagedFile.Name()
	cleanupStaged := true
	defer func() {
		if cleanupStaged {
			os.Remove(stagedPath)
		}
	}()

	_, copyErr := io.Copy(stagedFile, res.Body)
	closeErr := stagedFile.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	// os.CreateTemp creates the staged file with mode 0600. Preserve the
	// existing destination's mode when replacing it, otherwise fall back to
	// 0644 (honoring umask) to match the prior os.Create behavior instead of
	// silently tightening permissions to 0600.
	mode := os.FileMode(0o644)
	if info, statErr := os.Stat(hf.Destination); statErr == nil {
		mode = info.Mode().Perm()
	}
	if err := os.Chmod(stagedPath, mode); err != nil {
		return err
	}
	if err := os.Rename(stagedPath, hf.Destination); err != nil {
		return err
	}
	cleanupStaged = false
	return nil
}

func expectedStatusCode(props map[string]interface{}) int {
	rawExpectedCode := props[expectedCodeProperty]
	if rawExpectedCode == nil {
		return defaultExpectedStatusCode
	}

	switch expectedCode := rawExpectedCode.(type) {
	case int:
		return expectedCode
	case int8:
		return int(expectedCode)
	case int16:
		return int(expectedCode)
	case int32:
		return int(expectedCode)
	case int64:
		return boundedStatusCode(expectedCode)
	case uint:
		return int(expectedCode)
	case uint8:
		return int(expectedCode)
	case uint16:
		return int(expectedCode)
	case uint32:
		return int(expectedCode)
	case uint64:
		if expectedCode <= maxExpectedStatusCode {
			return int(expectedCode)
		}
	case float32:
		return integralFloatStatusCode(float64(expectedCode))
	case float64:
		return integralFloatStatusCode(expectedCode)
	case json.Number:
		parsedCode, err := strconv.ParseInt(expectedCode.String(), 10, 64)
		if err == nil {
			return boundedStatusCode(parsedCode)
		}
	}

	return defaultExpectedStatusCode
}

func boundedStatusCode(expectedCode int64) int {
	if expectedCode >= 0 && expectedCode <= maxExpectedStatusCode {
		return int(expectedCode)
	}

	return defaultExpectedStatusCode
}

func integralFloatStatusCode(expectedCode float64) int {
	parsedCode := int(expectedCode)
	if float64(parsedCode) == expectedCode && parsedCode >= 0 && parsedCode <= maxExpectedStatusCode {
		return parsedCode
	}

	return defaultExpectedStatusCode
}

func applyRequestHeaders(req *httpc.Request, headers interface{}) error {
	if headers == nil {
		return nil
	}

	switch typedHeaders := headers.(type) {
	case map[string]string:
		for headerName, headerValue := range typedHeaders {
			req.Header.Set(headerName, headerValue)
		}
	case map[string]interface{}:
		for headerName, rawHeaderValue := range typedHeaders {
			headerValues, err := normalizeHeaderValues(headerName, rawHeaderValue)
			if err != nil {
				return err
			}
			setHeaderValues(req.Header, headerName, headerValues)
		}
	case httpc.Header:
		req.Header = typedHeaders.Clone()
	default:
		return fmt.Errorf("headers property must be a map, got %T", headers)
	}

	return nil
}

func normalizeHeaderValues(headerName string, rawHeaderValue interface{}) ([]string, error) {
	switch headerValue := rawHeaderValue.(type) {
	case string:
		return []string{headerValue}, nil
	case []string:
		return headerValue, nil
	case []interface{}:
		values := make([]string, 0, len(headerValue))
		for _, rawValue := range headerValue {
			value, ok := rawValue.(string)
			if !ok {
				return nil, fmt.Errorf("header %q value must contain only strings", headerName)
			}
			values = append(values, value)
		}
		return values, nil
	default:
		return nil, fmt.Errorf("header %q value must be a string or string array", headerName)
	}
}

func setHeaderValues(headers httpc.Header, headerName string, headerValues []string) {
	headers.Del(headerName)
	for _, headerValue := range headerValues {
		headers.Add(headerName, headerValue)
	}
}

func (hf HTTPFile) Properties() (map[string]interface{}, error) {
	return hf.Props, nil
}

func (hf HTTPFile) Parse(id, source, destination, hash string, properties map[string]interface{}) (file.FileProvider, error) {
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

func init() {
	file.RegisterProvider(HTTPFile{})
}
