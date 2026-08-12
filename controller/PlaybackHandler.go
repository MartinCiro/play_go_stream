package controller

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

// PlaybackHandler maneja los endpoints de control de reproducción
type PlaybackHandler struct {
	config   *Config
	playback *PlaybackController
}

// NewPlaybackHandler crea un nuevo handler
func NewPlaybackHandler(config *Config, playback *PlaybackController) *PlaybackHandler {
	return &PlaybackHandler{
		config:   config,
		playback: playback,
	}
}

// RegisterRoutes registra las rutas en el mux
func (h *PlaybackHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/playback/state", h.handleState)
	mux.HandleFunc("/api/playback/play", h.handlePlay)
	mux.HandleFunc("/api/playback/pause", h.handlePause)
	mux.HandleFunc("/api/playback/stop", h.handleStop)
	mux.HandleFunc("/api/playback/next", h.handleNext)
	mux.HandleFunc("/api/playback/previous", h.handlePrevious)
	mux.HandleFunc("/api/playback/seek", h.handleSeek)
	mux.HandleFunc("/api/playback/queue", h.handleQueue)
	mux.HandleFunc("/api/playback/queue/add", h.handleQueueAdd)
	mux.HandleFunc("/api/playback/queue/clear", h.handleQueueClear)
	mux.HandleFunc("/api/playback/queue/remove", h.handleQueueRemove)
	mux.HandleFunc("/api/playback/play-now", h.handlePlayNow)
	mux.HandleFunc("/api/playback/repeat", h.handleRepeat)
	mux.HandleFunc("/api/playback/shuffle", h.handleShuffle)
}

// POST /api/playback/play
func (h *PlaybackHandler) handlePlay(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		ErrorResponse(w, http.StatusMethodNotAllowed, "Método no permitido")
		return
	}
	status, err := h.playback.Play()
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	SuccessResponse(w, status)
}

// POST /api/playback/pause
func (h *PlaybackHandler) handlePause(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		ErrorResponse(w, http.StatusMethodNotAllowed, "Método no permitido")
		return
	}
	status, err := h.playback.Pause()
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	SuccessResponse(w, status)
}

// POST /api/playback/stop
func (h *PlaybackHandler) handleStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		ErrorResponse(w, http.StatusMethodNotAllowed, "Método no permitido")
		return
	}
	status, err := h.playback.Stop()
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	SuccessResponse(w, status)
}

// POST /api/playback/next
func (h *PlaybackHandler) handleNext(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		ErrorResponse(w, http.StatusMethodNotAllowed, "Método no permitido")
		return
	}
	status, err := h.playback.Next()
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	SuccessResponse(w, status)
}

// POST /api/playback/previous
func (h *PlaybackHandler) handlePrevious(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		ErrorResponse(w, http.StatusMethodNotAllowed, "Método no permitido")
		return
	}
	status, err := h.playback.Previous()
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	SuccessResponse(w, status)
}

// GET /api/playback/state
func (h *PlaybackHandler) handleState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		ErrorResponse(w, http.StatusMethodNotAllowed, "Método no permitido")
		return
	}
	SuccessResponse(w, h.playback.GetStatus())
}

// POST /api/playback/seek { "position": 45.5 }
func (h *PlaybackHandler) handleSeek(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		ErrorResponse(w, http.StatusMethodNotAllowed, "Método no permitido")
		return
	}

	var req struct {
		Position float64 `json:"position"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorResponse(w, http.StatusBadRequest, "JSON inválido")
		return
	}

	status, err := h.playback.Seek(req.Position)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	SuccessResponse(w, status)
}

// POST /api/playback/queue { "song_ids": ["id1", "id2", ...] }
func (h *PlaybackHandler) handleQueue(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		// GET: retornar la cola actual
		status := h.playback.GetStatus()
		SuccessResponse(w, map[string]interface{}{
			"queue":         status.Queue,
			"current_index": status.QueueIndex,
			"length":        status.QueueLength,
		})
		return
	}

	if r.Method != http.MethodPost {
		ErrorResponse(w, http.StatusMethodNotAllowed, "Método no permitido")
		return
	}

	var req struct {
		SongIDs []string `json:"song_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorResponse(w, http.StatusBadRequest, "JSON inválido")
		return
	}

	status, err := h.playback.SetQueue(req.SongIDs)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	SuccessResponse(w, status)
}

// POST /api/playback/queue/add { "song_id": "id" }
func (h *PlaybackHandler) handleQueueAdd(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		ErrorResponse(w, http.StatusMethodNotAllowed, "Método no permitido")
		return
	}

	var req struct {
		SongID string `json:"song_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorResponse(w, http.StatusBadRequest, "JSON inválido")
		return
	}

	status, err := h.playback.AddToQueue(req.SongID)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	SuccessResponse(w, status)
}

// POST /api/playback/play-now { "song_id": "id" }
func (h *PlaybackHandler) handlePlayNow(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		ErrorResponse(w, http.StatusMethodNotAllowed, "Método no permitido")
		return
	}

	var req struct {
		SongID string `json:"song_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorResponse(w, http.StatusBadRequest, "JSON inválido")
		return
	}

	status, err := h.playback.PlayNow(req.SongID)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	SuccessResponse(w, status)
}

// POST /api/playback/queue/clear
func (h *PlaybackHandler) handleQueueClear(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		ErrorResponse(w, http.StatusMethodNotAllowed, "Método no permitido")
		return
	}
	status, err := h.playback.ClearQueue()
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	SuccessResponse(w, status)
}

// DELETE /api/playback/queue/remove?index=2
func (h *PlaybackHandler) handleQueueRemove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete && r.Method != http.MethodPost {
		ErrorResponse(w, http.StatusMethodNotAllowed, "Método no permitido")
		return
	}

	indexStr := r.URL.Query().Get("index")
	if indexStr == "" {
		ErrorResponse(w, http.StatusBadRequest, "Parámetro 'index' requerido")
		return
	}

	index, err := strconv.Atoi(indexStr)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, "Índice inválido")
		return
	}

	status, err := h.playback.RemoveFromQueue(index)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	SuccessResponse(w, status)
}

// POST /api/playback/repeat { "mode": "none|one|all" } o GET para toggle
func (h *PlaybackHandler) handleRepeat(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		// Toggle en GET
		status, err := h.playback.ToggleRepeat()
		if err != nil {
			ErrorResponse(w, http.StatusBadRequest, err.Error())
			return
		}
		SuccessResponse(w, status)
		return
	}

	if r.Method != http.MethodPost {
		ErrorResponse(w, http.StatusMethodNotAllowed, "Método no permitido")
		return
	}

	var req struct {
		Mode string `json:"mode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorResponse(w, http.StatusBadRequest, "JSON inválido")
		return
	}

	// Permitir también pasar mode como string directo
	if req.Mode == "" {
		// Intentar leer como string
		var mode string
		if err := json.NewDecoder(strings.NewReader("")).Decode(&mode); err == nil && mode != "" {
			req.Mode = mode
		}
	}

	status, err := h.playback.SetRepeatMode(req.Mode)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	SuccessResponse(w, status)
}

// GET /api/playback/shuffle -> toggle
// POST /api/playback/shuffle { "enabled": true|false }
func (h *PlaybackHandler) handleShuffle(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		// Toggle simple
		status, err := h.playback.ToggleShuffle()
		if err != nil {
			ErrorResponse(w, http.StatusBadRequest, err.Error())
			return
		}
		SuccessResponse(w, status)
		return
	}

	if r.Method != http.MethodPost {
		ErrorResponse(w, http.StatusMethodNotAllowed, "Método no permitido")
		return
	}

	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorResponse(w, http.StatusBadRequest, "JSON inválido")
		return
	}

	status, err := h.playback.SetShuffle(req.Enabled)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	SuccessResponse(w, status)
}
