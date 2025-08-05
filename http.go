package main

import (
	"encoding/json"
	"net/http"
)

func RespondJson(w http.ResponseWriter, code int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)

	json.NewEncoder(w).Encode(data)
}
