package controller

import (
	"encoding/json"
	"net/http"
)

// Handlers agrupa los handlers HTTP con sus dependencias
type Handlers struct {
	lib      *Library
	streamer *Streamer
	log      *Logger
}

// NewHandlers crea el set de handlers con DI
func NewHandlers(lib *Library, streamer *Streamer, log *Logger) *Handlers {
	return &Handlers{lib: lib, streamer: streamer, log: log}
}

// --- Helpers de respuesta ---

type apiResponse struct {
	OK    bool        `json:"ok"`
	Data  interface{} `json:"data,omitempty"`
	Error string      `json:"error,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(payload)
}

func writeOK(w http.ResponseWriter, data interface{}) {
	writeJSON(w, http.StatusOK, apiResponse{OK: true, Data: data})
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, apiResponse{OK: false, Error: msg})
}

// --- Endpoints ---

// Health check simple
func (h *Handlers) Health(w http.ResponseWriter, r *http.Request) {
	writeOK(w, map[string]interface{}{
		"status":  "up",
		"service": "go_stream",
		"tracks":  h.lib.Count(),
	})
}

// ListTracks devuelve toda la biblioteca
func (h *Handlers) ListTracks(w http.ResponseWriter, r *http.Request) {
	writeOK(w, h.lib.All())
}

// GetTrack devuelve metadata de un track específico
func (h *Handlers) GetTrack(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	track, ok := h.lib.Get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "track not found")
		return
	}
	writeOK(w, track)
}

// StreamTrack sirve el audio del track
func (h *Handlers) StreamTrack(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	h.streamer.ServeHTTP(w, r, id)
}

// Rescan dispara un nuevo escaneo
func (h *Handlers) Rescan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "use POST")
		return
	}
	go func() {
		if err := h.lib.Scan(); err != nil {
			h.log.Error("Rescan falló: %v", err)
		}
	}()
	writeOK(w, map[string]string{"status": "rescan started"})
}
