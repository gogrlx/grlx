//go:build linux

package main

import (
	_ "github.com/gogrlx/grlx/v2/internal/ingredients/service/openrc"
	_ "github.com/gogrlx/grlx/v2/internal/ingredients/service/systemd"
)
