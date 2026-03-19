package natsapi

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gogrlx/grlx/v2/internal/config"
)

// MaxFileChunkSize is the maximum number of bytes returned in a single
// files.get response. Clients requesting files larger than this must
// use offset-based chunking. The limit ensures responses stay well
// within NATS max payload (typically 1 MB).
const MaxFileChunkSize = 512 * 1024 // 512 KB

// FileGetRequest is the input for files.get.
type FileGetRequest struct {
	// Path is the file path relative to the recipe directory.
	Path string `json:"path"`
	// Offset is the byte offset to start reading from (default 0).
	Offset int64 `json:"offset,omitempty"`
	// Limit is the maximum number of bytes to return (default/max: MaxFileChunkSize).
	Limit int64 `json:"limit,omitempty"`
}

// FileGetResponse is the output for files.get.
type FileGetResponse struct {
	// Path is the requested file path (echoed back).
	Path string `json:"path"`
	// Size is the total file size in bytes.
	Size int64 `json:"size"`
	// Offset is the byte offset of the returned data.
	Offset int64 `json:"offset"`
	// Length is the number of bytes in Data.
	Length int64 `json:"length"`
	// Data is the base64-encoded file content for this chunk.
	Data string `json:"data"`
	// Done is true when this chunk includes the end of the file.
	Done bool `json:"done"`
}

func handleFilesGet(params json.RawMessage) (any, error) {
	var req FileGetRequest
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	if req.Path == "" {
		return nil, fmt.Errorf("file path is required")
	}

	recipeDir := config.RecipeDir
	if recipeDir == "" {
		return nil, fmt.Errorf("recipe directory not configured")
	}

	// Resolve and validate the path to prevent traversal attacks.
	fullPath := filepath.Join(recipeDir, req.Path)
	fullPath = filepath.Clean(fullPath)

	absRecipeDir, err := filepath.Abs(recipeDir)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve recipe directory: %w", err)
	}
	absFullPath, err := filepath.Abs(fullPath)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve file path: %w", err)
	}
	if !strings.HasPrefix(absFullPath, absRecipeDir+string(filepath.Separator)) && absFullPath != absRecipeDir {
		return nil, fmt.Errorf("invalid file path: path traversal detected")
	}

	fi, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("file not found: %s", req.Path)
		}
		return nil, fmt.Errorf("cannot access file: %w", err)
	}
	if fi.IsDir() {
		return nil, fmt.Errorf("path is a directory: %s", req.Path)
	}

	totalSize := fi.Size()
	offset := req.Offset
	if offset < 0 {
		offset = 0
	}
	if offset > totalSize {
		offset = totalSize
	}

	limit := req.Limit
	if limit <= 0 || limit > MaxFileChunkSize {
		limit = MaxFileChunkSize
	}

	// Calculate how many bytes to read.
	remaining := totalSize - offset
	readSize := limit
	if readSize > remaining {
		readSize = remaining
	}

	f, err := os.Open(fullPath)
	if err != nil {
		return nil, fmt.Errorf("cannot open file: %w", err)
	}
	defer f.Close()

	if offset > 0 {
		if _, err := f.Seek(offset, 0); err != nil {
			return nil, fmt.Errorf("cannot seek file: %w", err)
		}
	}

	buf := make([]byte, readSize)
	n, err := f.Read(buf)
	if err != nil && n == 0 {
		return nil, fmt.Errorf("cannot read file: %w", err)
	}
	buf = buf[:n]

	return FileGetResponse{
		Path:   req.Path,
		Size:   totalSize,
		Offset: offset,
		Length: int64(n),
		Data:   base64.StdEncoding.EncodeToString(buf),
		Done:   offset+int64(n) >= totalSize,
	}, nil
}
