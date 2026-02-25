package file

import (
	"github.com/gogrlx/grlx/v2/internal/ingredients/file/http"
	"github.com/gogrlx/grlx/v2/internal/ingredients/file/local"
	"github.com/gogrlx/grlx/v2/types"
)

func init() {
	provMap = make(map[string]types.FileProvider)
	RegisterProvider(http.HTTPFile{})
	// RegisterProvider(s3.S3File{})
	RegisterProvider(local.LocalFile{})
}
