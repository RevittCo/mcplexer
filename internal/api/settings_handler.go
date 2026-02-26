package api

import (
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/revittco/mcplexer/internal/config"
)

type settingsHandler struct {
	svc *config.SettingsService
}

type settingsResponse struct {
	Settings            config.Settings   `json:"settings"`
	BuiltinToolDefaults map[string]string `json:"builtin_tool_defaults"`
}

func (h *settingsHandler) get(w http.ResponseWriter, r *http.Request) {
	settings := h.svc.Load(r.Context())
	writeJSON(w, http.StatusOK, settingsResponse{
		Settings:            settings,
		BuiltinToolDefaults: config.BuiltinToolDefaults(),
	})
}

func (h *settingsHandler) update(w http.ResponseWriter, r *http.Request) {
	var settings config.Settings
	if err := decodeJSON(r, &settings); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if err := h.svc.Save(r.Context(), settings); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Apply log level change at runtime.
	applyLogLevel(settings.LogLevel)

	// Return the saved settings (re-load to include env overrides).
	saved := h.svc.Load(r.Context())
	writeJSON(w, http.StatusOK, settingsResponse{
		Settings:            saved,
		BuiltinToolDefaults: config.BuiltinToolDefaults(),
	})
}

func applyLogLevel(level string) {
	var l slog.Level
	switch strings.ToLower(level) {
	case "debug":
		l = slog.LevelDebug
	case "warn":
		l = slog.LevelWarn
	case "error":
		l = slog.LevelError
	default:
		l = slog.LevelInfo
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: l,
	})))
}
