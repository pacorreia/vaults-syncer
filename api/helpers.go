package api

import (
	"encoding/json"
	"net/http"
)

// jsonOK writes a 200 response with a JSON body.
func jsonOK(w http.ResponseWriter, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(body)
}

// jsonError writes an error response with the given HTTP status code.
func jsonError(w http.ResponseWriter, message string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}
