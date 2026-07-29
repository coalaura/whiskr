package main

import "net/http"

type Usage struct {
	Total   float64 `json:"total"`
	Daily   float64 `json:"daily"`
	Weekly  float64 `json:"weekly"`
	Monthly float64 `json:"monthly"`
}

func HandleUsage(w http.ResponseWriter, r *http.Request) {
	client := OpenRouterClient(nil)

	current, err := client.GetCurrentApiKey(r.Context())
	if err != nil {
		log.Warnln(err)

		RespondJson(w, http.StatusInternalServerError, map[string]any{
			"error": err.Error(),
		})

		return
	}

	RespondJson(w, http.StatusOK, Usage{
		Total:   current.Usage,
		Daily:   current.UsageDaily,
		Weekly:  current.UsageWeekly,
		Monthly: current.UsageMonthly,
	})
}
