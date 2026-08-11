package controller

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// MusicHTTP expone una API REST para el portal web
type MusicHTTP struct {
	music *MusicService
	log   *Logger
	port  int
}

// NewMusicHTTP crea el servidor HTTP
func NewMusicHTTP(music *MusicService, log *Logger, port int) *MusicHTTP {
	return &MusicHTTP{
		music: music,
		log:   log,
		port:  port,
	}
}

// Start inicia el servidor HTTP en segundo plano
func (h *MusicHTTP) Start() error {
	mux := http.NewServeMux()

	// Endpoints básicos
	mux.HandleFunc("/api/tracks", h.handleTracks)
	mux.HandleFunc("/api/state", h.handleState)
	mux.HandleFunc("/api/play/", h.handlePlay)
	mux.HandleFunc("/api/pause", h.handlePause)
	mux.HandleFunc("/api/resume", h.handleResume)
	mux.HandleFunc("/api/stop", h.handleStop)
	mux.HandleFunc("/api/volume", h.handleVolume)

	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", h.port),
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	if h.log != nil {
		h.log.Info(fmt.Sprintf("Servidor HTTP de música iniciado en puerto %d", h.port))
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			if h.log != nil {
				h.log.Error(fmt.Sprintf("Servidor HTTP cayó: %v", err), "MusicHTTP")
			}
		}
	}()

	return nil
}

func (h *MusicHTTP) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *MusicHTTP) handleTracks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "método no permitido"})
		return
	}
	tracks := h.music.ListTracks()
	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"count":  len(tracks),
		"tracks": tracks,
	})
}

func (h *MusicHTTP) handleState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "método no permitido"})
		return
	}
	state := h.music.GetState()
	h.writeJSON(w, http.StatusOK, state)
}

func (h *MusicHTTP) handlePlay(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "método no permitido"})
		return
	}

	// Extraer ID de la URL: /api/play/abcd1234
	trackID := r.URL.Path[len("/api/play/"):]
	if trackID == "" {
		h.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ID de canción requerido"})
		return
	}

	if err := h.music.Play(trackID); err != nil {
		h.writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "track_id": trackID})
}

func (h *MusicHTTP) handlePause(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "método no permitido"})
		return
	}
	if err := h.music.Pause(); err != nil {
		h.writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	h.writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *MusicHTTP) handleResume(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "método no permitido"})
		return
	}
	if err := h.music.Resume(); err != nil {
		h.writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	h.writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *MusicHTTP) handleStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "método no permitido"})
		return
	}
	if err := h.music.Stop(); err != nil {
		h.writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	h.writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *MusicHTTP) handleVolume(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "método no permitido"})
		return
	}

	var body struct {
		Volume float64 `json:"volume"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		h.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "JSON inválido"})
		return
	}

	if err := h.music.SetVolume(body.Volume); err != nil {
		h.writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "volume": fmt.Sprintf("%.0f%%", body.Volume*100)})
}

// Helper para convertir string a int
func (h *MusicHTTP) parseInt(s string, def int) int {
	v, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return v
}
