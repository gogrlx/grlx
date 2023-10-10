package file

import (
	"github.com/gogrlx/grlx/ingredients/file/http"
	"github.com/gogrlx/grlx/ingredients/file/local"
	"github.com/gogrlx/grlx/ingredients/file/s3"
	"github.com/gogrlx/grlx/types"
)

func init() {
	provMap = make(map[string]types.FileProvider)
	RegisterProvider(http.HTTPFile{})
	RegisterProvider(s3.S3File{})
	RegisterProvider(local.LocalFile{})
}
