package servicesettings

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/kayushkin/llm-bridge/msg"
)

// Handler serves a Registry the same way in every service:
//
//	GET  {mount}           msg.ServiceSettings
//	PUT  {mount}/{key}     msg.ServiceSettingUpdate → the updated msg.ServiceSettings
//
// mount is the path the handler is registered at, "/settings" by convention.
// A refusal is {"error": "…"} with 404 for a key nobody declared, 409 for a
// setting that is not stored by the service, 400 for a value that does not
// parse or that the setting's validator refused, and 502 when the validator
// could not ask the owner of what the value names. Nothing is stored on a
// refusal.
//
// Who may call it is the service's own business: wrap the handler in whatever
// gate the service's operator routes sit behind.
func Handler(registry *Registry, mount string) http.Handler {
	mount = strings.TrimRight(mount, "/")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rest := strings.Trim(strings.TrimPrefix(r.URL.Path, mount), "/")
		switch {
		case rest == "" && r.Method == http.MethodGet:
			writeJSON(w, http.StatusOK, registry.Describe())
		case rest != "" && !strings.Contains(rest, "/") && r.Method == http.MethodPut:
			var update msg.ServiceSettingUpdate
			decoder := json.NewDecoder(r.Body)
			decoder.DisallowUnknownFields()
			if err := decoder.Decode(&update); err != nil {
				writeError(w, http.StatusBadRequest, "body must be {\"value\": \"…\"}: "+err.Error())
				return
			}
			if err := registry.Set(rest, update.Value); err != nil {
				writeError(w, statusOf(err), err.Error())
				return
			}
			writeJSON(w, http.StatusOK, registry.Describe())
		default:
			writeError(w, http.StatusMethodNotAllowed, "GET "+mount+" or PUT "+mount+"/{key}")
		}
	})
}

func statusOf(err error) int {
	switch {
	case errors.Is(err, ErrUnknownSetting):
		return http.StatusNotFound
	case errors.Is(err, ErrSettingNotEditable):
		return http.StatusConflict
	case errors.Is(err, ErrInvalidSettingValue):
		return http.StatusBadRequest
	case errors.Is(err, ErrSettingOwnerUnavailable):
		return http.StatusBadGateway
	default:
		return http.StatusInternalServerError
	}
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
