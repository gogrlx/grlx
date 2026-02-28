package main

import (
	_ "github.com/gogrlx/grlx/v2/internal/ingredients/cmd"
	_ "github.com/gogrlx/grlx/v2/internal/ingredients/file"
	_ "github.com/gogrlx/grlx/v2/internal/ingredients/file/http"
	_ "github.com/gogrlx/grlx/v2/internal/ingredients/file/local"
	_ "github.com/gogrlx/grlx/v2/internal/ingredients/group"
	_ "github.com/gogrlx/grlx/v2/internal/ingredients/pkg"
	_ "github.com/gogrlx/grlx/v2/internal/ingredients/service/systemd"
	_ "github.com/gogrlx/grlx/v2/internal/ingredients/user"
)
