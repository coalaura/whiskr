package main

import (
	"encoding/json"
	"net/http"
)

type TokenizeRequest struct {
	String string `json:"string"`
}

func HandleTokenize(tokenizer *Tokenizer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		debug("parsing tokenize")

		var raw TokenizeRequest

		if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
			RespondJson(w, http.StatusBadRequest, map[string]any{
				"error": err.Error(),
			})

			return
		}

		tokens := tokenizer.Encode(raw.String)

		RespondJson(w, http.StatusOK, map[string]any{
			"tokens": len(tokens),
		})
	}
}
