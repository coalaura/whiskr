package main

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

func HandleUserSetting(w http.ResponseWriter, r *http.Request) {
	user := GetAuthenticatedUser(r)
	if user == nil {
		w.WriteHeader(http.StatusUnauthorized)

		return
	}

	setting := chi.URLParam(r, "setting")

	switch setting {
	case "favorites":
		var favorites []string

		err := json.NewDecoder(r.Body).Decode(&favorites)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)

			return
		}

		settings.SetFavorites(user.Username, favorites)
	default:
		w.WriteHeader(http.StatusBadRequest)

		return
	}

	w.WriteHeader(http.StatusOK)
}
