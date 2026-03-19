// Package farmer implements a FileProvider that fetches recipe files
// from the farmer over NATS instead of HTTP. Recipes reference files
// with farmer:// URLs (e.g. "farmer://configs/nginx.conf"), which
// resolve to recipe directory paths on the farmer.
//
// This provider requires a NATS connection registered via
// RegisterNatsConn before use. The farmer must have the
// grlx.api.files.get handler subscribed (see natsapi/files.go).
package farmer

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/gogrlx/grlx/v2/internal/ingredients/file"
	"github.com/gogrlx/grlx/v2/internal/ingredients/file/hashers"
	"github.com/gogrlx/grlx/v2/internal/log"
)

const (
	// NATSSubject is the NATS subject for file requests.
	NATSSubject = "grlx.api.files.get"

	// RequestTimeout is the timeout for each NATS file chunk request.
	RequestTimeout = 30 * time.Second

	// MaxChunkSize matches the farmer-side MaxFileChunkSize (512 KB).
	MaxChunkSize int64 = 512 * 1024
)

var natsConn *nats.Conn

// RegisterNatsConn sets the NATS connection used by the farmer provider.
// This must be called during sprout startup.
func RegisterNatsConn(nc *nats.Conn) {
	natsConn = nc
}

// fileGetRequest is the input for the files.get NATS handler.
type fileGetRequest struct {
	Path   string `json:"path"`
	Offset int64  `json:"offset,omitempty"`
	Limit  int64  `json:"limit,omitempty"`
}

// fileGetResponse is the output from the files.get NATS handler.
type fileGetResponse struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	Offset int64  `json:"offset"`
	Length int64  `json:"length"`
	Data   string `json:"data"`
	Done   bool   `json:"done"`
}

// natsResponse is the envelope returned by NATS API handlers.
type natsResponse struct {
	Result json.RawMessage `json:"result,omitempty"`
	Error  string          `json:"error,omitempty"`
}

// FarmerFile implements file.FileProvider for farmer:// URLs.
type FarmerFile struct {
	ID          string
	Source      string
	Destination string
	Hash        string
	Props       map[string]interface{}
}

// extractPath strips the "farmer://" prefix from the source URL.
func extractPath(source string) string {
	return strings.TrimPrefix(source, "farmer://")
}

// Download fetches the file from the farmer over NATS, handling
// chunked transfers for files larger than MaxChunkSize.
func (ff FarmerFile) Download(ctx context.Context) error {
	if natsConn == nil {
		return fmt.Errorf("farmer provider: NATS connection not registered")
	}

	filePath := extractPath(ff.Source)
	if filePath == "" {
		return fmt.Errorf("farmer provider: empty file path in source %q", ff.Source)
	}

	dest, err := os.Create(ff.Destination)
	if err != nil {
		return fmt.Errorf("farmer provider: cannot create destination %q: %w", ff.Destination, err)
	}
	defer func() {
		dest.Close()
		// Clean up on error — caller will retry or report.
	}()

	var offset int64
	for {
		req := fileGetRequest{
			Path:   filePath,
			Offset: offset,
			Limit:  MaxChunkSize,
		}

		data, err := json.Marshal(req)
		if err != nil {
			os.Remove(ff.Destination)
			return fmt.Errorf("farmer provider: marshal request: %w", err)
		}

		// Use context deadline if available, otherwise default timeout.
		timeout := RequestTimeout
		if deadline, ok := ctx.Deadline(); ok {
			remaining := time.Until(deadline)
			if remaining < timeout {
				timeout = remaining
			}
		}

		msg, err := natsConn.Request(NATSSubject, data, timeout)
		if err != nil {
			os.Remove(ff.Destination)
			return fmt.Errorf("farmer provider: NATS request for %q (offset %d): %w", filePath, offset, err)
		}

		var resp natsResponse
		if err := json.Unmarshal(msg.Data, &resp); err != nil {
			os.Remove(ff.Destination)
			return fmt.Errorf("farmer provider: unmarshal response: %w", err)
		}
		if resp.Error != "" {
			os.Remove(ff.Destination)
			return fmt.Errorf("farmer provider: server error: %s", resp.Error)
		}

		var fileResp fileGetResponse
		if err := json.Unmarshal(resp.Result, &fileResp); err != nil {
			os.Remove(ff.Destination)
			return fmt.Errorf("farmer provider: unmarshal file response: %w", err)
		}

		decoded, err := base64.StdEncoding.DecodeString(fileResp.Data)
		if err != nil {
			os.Remove(ff.Destination)
			return fmt.Errorf("farmer provider: decode chunk data: %w", err)
		}

		if _, err := dest.Write(decoded); err != nil {
			os.Remove(ff.Destination)
			return fmt.Errorf("farmer provider: write chunk to %q: %w", ff.Destination, err)
		}

		log.Tracef("farmer provider: fetched %q chunk offset=%d length=%d done=%v",
			filePath, fileResp.Offset, fileResp.Length, fileResp.Done)

		if fileResp.Done {
			break
		}

		offset = fileResp.Offset + fileResp.Length

		// Check context cancellation between chunks.
		select {
		case <-ctx.Done():
			os.Remove(ff.Destination)
			return ctx.Err()
		default:
		}
	}

	return nil
}

// Properties returns the provider's properties.
func (ff FarmerFile) Properties() (map[string]interface{}, error) {
	return ff.Props, nil
}

// Parse creates a new FarmerFile from the given parameters.
func (ff FarmerFile) Parse(id, source, destination, hash string, properties map[string]interface{}) (file.FileProvider, error) {
	if properties == nil {
		properties = make(map[string]interface{})
	}
	return FarmerFile{
		ID:          id,
		Source:      source,
		Destination: destination,
		Hash:        hash,
		Props:       properties,
	}, nil
}

// Protocols returns the protocol schemes handled by this provider.
func (ff FarmerFile) Protocols() []string {
	return []string{"farmer"}
}

// Verify checks whether the destination file exists and matches the expected hash.
func (ff FarmerFile) Verify(ctx context.Context) (bool, error) {
	hashType := ""
	if ff.Props["hashType"] == nil {
		hashType = hashers.GuessHashType(ff.Hash)
	} else if ht, ok := ff.Props["hashType"].(string); !ok {
		hashType = hashers.GuessHashType(ff.Hash)
	} else {
		hashType = ht
	}
	cf := hashers.CacheFile{
		ID:          ff.ID,
		Destination: ff.Destination,
		Hash:        ff.Hash,
		HashType:    hashType,
	}
	return cf.Verify(ctx)
}

func init() {
	file.RegisterProvider(FarmerFile{})
}
