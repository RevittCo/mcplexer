package api

import (
	"encoding/json"
	"net/http"
	"time"
)

var startTime = time.Now()

type healthResponse struct {
	Status        string `json:"status"`
	Version       string `json:"version"`
	UptimeSeconds int    `json:"uptime_seconds"`
	Mode          string `json:"mode"`
}

func healthCheck(w http.ResponseWriter, _ *http.Request) {
	resp := healthResponse{
		Status:        "ok",
		Version:       "0.1.0",
		UptimeSeconds: int(time.Since(startTime).Seconds()),
		Mode:          "http",
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
