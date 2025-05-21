package handlers

import (
	"net/http"

	log "github.com/taigrr/log-socket/log"

	"github.com/gogrlx/grlx/types"
	"github.com/gogrlx/grlx/web/templates"
)

var buildVersion types.Version

func SetBuildVersion(v types.Version) {
	buildVersion = v
}

func GetVersion(w http.ResponseWriter, r *http.Request) {
	name := "GRLX User" // Or get a name from request parameters if you want to be fancy
	component := templates.Greeting(name)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	err := component.Render(r.Context(), w)
	if err != nil {
		http.Error(w, "Failed to render template", http.StatusInternalServerError)
		log.Errorf("Error rendering template: %v", err) // Assuming a logger `log` is available
		return
	}
}
