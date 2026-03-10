package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	log "github.com/gogrlx/grlx/v2/internal/log"

	apitypes "github.com/gogrlx/grlx/v2/internal/api/types"
	"github.com/gogrlx/grlx/v2/internal/props"
)

// propRequest is the JSON body for setting a property value.
type propRequest struct {
	Value string `json:"value"`
}

// GetAllProps returns all non-expired properties for a sprout.
// GET /props/{sproutID}
func GetAllProps(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	sproutID := vars["sproutID"]
	if sproutID == "" {
		w.WriteHeader(http.StatusBadRequest)
		jw, _ := json.Marshal(apitypes.Inline{Success: false})
		w.Write(jw)
		return
	}
	allProps := props.GetProps(sproutID)
	if allProps == nil {
		allProps = make(map[string]interface{})
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	jw, err := json.Marshal(allProps)
	if err != nil {
		log.Errorf("failed to marshal props for %s: %v", sproutID, err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Write(jw)
}

// GetProp returns a single property value for a sprout.
// GET /props/{sproutID}/{name}
func GetProp(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	sproutID := vars["sproutID"]
	name := vars["name"]
	if sproutID == "" || name == "" {
		w.WriteHeader(http.StatusBadRequest)
		jw, _ := json.Marshal(apitypes.Inline{Success: false})
		w.Write(jw)
		return
	}
	value := props.GetStringProp(sproutID, name)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	resp := map[string]string{
		"sproutID": sproutID,
		"name":     name,
		"value":    value,
	}
	jw, err := json.Marshal(resp)
	if err != nil {
		log.Errorf("failed to marshal prop %s/%s: %v", sproutID, name, err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Write(jw)
}

// SetProp creates or updates a property for a sprout.
// PUT /props/{sproutID}/{name}
func SetProp(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	sproutID := vars["sproutID"]
	name := vars["name"]
	if sproutID == "" || name == "" {
		w.WriteHeader(http.StatusBadRequest)
		jw, _ := json.Marshal(apitypes.Inline{Success: false})
		w.Write(jw)
		return
	}
	var req propRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := props.SetProp(sproutID, name, req.Value); err != nil {
		log.Errorf("failed to set prop %s/%s: %v", sproutID, name, err)
		w.WriteHeader(http.StatusBadRequest)
		jw, _ := json.Marshal(apitypes.Inline{Success: false, Error: err})
		w.Write(jw)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	jw, _ := json.Marshal(apitypes.Inline{Success: true})
	w.Write(jw)
}

// DeleteProp removes a property for a sprout.
// DELETE /props/{sproutID}/{name}
func DeleteProp(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	sproutID := vars["sproutID"]
	name := vars["name"]
	if sproutID == "" || name == "" {
		w.WriteHeader(http.StatusBadRequest)
		jw, _ := json.Marshal(apitypes.Inline{Success: false})
		w.Write(jw)
		return
	}
	if err := props.DeleteProp(sproutID, name); err != nil {
		log.Errorf("failed to delete prop %s/%s: %v", sproutID, name, err)
		w.WriteHeader(http.StatusBadRequest)
		jw, _ := json.Marshal(apitypes.Inline{Success: false, Error: err})
		w.Write(jw)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	jw, _ := json.Marshal(apitypes.Inline{Success: true})
	w.Write(jw)
}
