package main

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

func HandleUserSetting(w http.ResponseWriter, r *http.Request) {
	username, ok := settingsUsername(r)
	if !ok {
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

		valid := true

		modelMx.RLock()

		for _, favorite := range favorites {
			if _, ok := ModelIDMap[favorite]; !IsModelShortID(favorite) || !ok {
				valid = false

				debug("invalid favorite model %q", favorite)

				break
			}
		}

		modelMx.RUnlock()

		if !valid {
			w.WriteHeader(http.StatusBadRequest)

			return
		}

		settings.SetFavorites(username, favorites)
	case "cost_limit", "retry_max_retries", "retry_initial_delay", "retry_max_delay", "tts_model", "tts_voice", "theme", "provider_sorting", "model_benchmark", "image_resize", "max_images":
		var value json.RawMessage

		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))

		err := decoder.Decode(&value)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)

			return
		}

		err = settings.SetPreference(username, setting, value)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)

			return
		}
	default:
		w.WriteHeader(http.StatusBadRequest)

		return
	}

	w.WriteHeader(http.StatusOK)
}

func settingsUsername(r *http.Request) (string, bool) {
	user := GetAuthenticatedUser(r)
	if user != nil {
		return user.Username, true
	}

	return "", !env.Authentication.Enabled
}
