package controller

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
)

// StreamHandler maneja las peticiones HTTP del servidor de música
type StreamHandler struct {
	config          *Config
	musicService    *MusicService
	playbackHandler *PlaybackHandler
}

// NewStreamHandler crea un nuevo handler
func NewStreamHandler(config *Config, musicService *MusicService, playback *PlaybackController) *StreamHandler {
	return &StreamHandler{
		config:          config,
		musicService:    musicService,
		playbackHandler: NewPlaybackHandler(config, playback),
	}
}

// Start inicia el servidor HTTP
func (h *StreamHandler) Start() error {
	mux := http.NewServeMux()

	// API endpoints de biblioteca
	mux.HandleFunc("/api/library", h.handleLibrary)
	mux.HandleFunc("/api/search", h.handleSearch)
	mux.HandleFunc("/api/song/", h.handleSongInfo)
	mux.HandleFunc("/api/status", h.handleStatus)

	// Streaming endpoint
	mux.HandleFunc("/stream/", h.handleStream)

	// ← NUEVO: Registrar rutas de playback
	h.playbackHandler.RegisterRoutes(mux)

	// CORS middleware wrapper
	handler := h.corsMiddleware(mux)

	addr := ":" + h.config.Port

	if h.config.Log != nil {
		h.config.Log.Comentario("INFO", fmt.Sprintf("Servidor HTTP escuchando en %s", addr))
	}

	return http.ListenAndServe(addr, handler)
}

// corsMiddleware añade headers CORS para permitir peticiones desde otros orígenes
func (h *StreamHandler) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Range")
		w.Header().Set("Access-Control-Expose-Headers", "Content-Length, Content-Range, Accept-Ranges")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Validar IP si hay lista blanca
		if len(h.config.AllowedIPs) > 0 {
			ip := extractIP(r.RemoteAddr)
			if !h.config.IsIPAllowed(ip) {
				if h.config.Log != nil {
					h.config.Log.Comentario("WARNING", fmt.Sprintf("IP no autorizada: %s", ip))
				}
				ErrorResponse(w, http.StatusForbidden, "IP no autorizada")
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

// handleLibrary retorna la lista completa de canciones
func (h *StreamHandler) handleLibrary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		ErrorResponse(w, http.StatusMethodNotAllowed, "Método no permitido")
		return
	}

	songs := h.musicService.GetAll()
	SuccessResponse(w, map[string]interface{}{
		"count": len(songs),
		"songs": songs,
	})
}

// handleSearch busca canciones por query
func (h *StreamHandler) handleSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		ErrorResponse(w, http.StatusMethodNotAllowed, "Método no permitido")
		return
	}

	query := r.URL.Query().Get("q")
	if query == "" {
		ErrorResponse(w, http.StatusBadRequest, "Parámetro 'q' requerido")
		return
	}

	songs := h.musicService.Search(query)
	SuccessResponse(w, map[string]interface{}{
		"query":   query,
		"count":   len(songs),
		"results": songs,
	})
}

// handleSongInfo retorna información de una canción específica
func (h *StreamHandler) handleSongInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		ErrorResponse(w, http.StatusMethodNotAllowed, "Método no permitido")
		return
	}

	// Extraer ID de la URL: /api/song/{id}
	id := strings.TrimPrefix(r.URL.Path, "/api/song/")
	if id == "" {
		ErrorResponse(w, http.StatusBadRequest, "ID de canción requerido")
		return
	}

	song, exists := h.musicService.GetByID(id)
	if !exists {
		ErrorResponse(w, http.StatusNotFound, "Canción no encontrada")
		return
	}

	SuccessResponse(w, song)
}

// handleStatus retorna información del servidor
func (h *StreamHandler) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		ErrorResponse(w, http.StatusMethodNotAllowed, "Método no permitido")
		return
	}

	SuccessResponse(w, map[string]interface{}{
		"server":      "Go Stream",
		"status":      "running",
		"total_songs": h.musicService.Count(),
		"music_path":  h.config.MusicPath,
	})
}

// handleStream hace streaming de un archivo de audio con soporte para Range requests
func (h *StreamHandler) handleStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		ErrorResponse(w, http.StatusMethodNotAllowed, "Método no permitido")
		return
	}

	// Extraer ID de la URL: /stream/{id}
	id := strings.TrimPrefix(r.URL.Path, "/stream/")
	if id == "" {
		ErrorResponse(w, http.StatusBadRequest, "ID de canción requerido")
		return
	}

	filePath, err := h.musicService.GetFilePath(id)
	if err != nil {
		ErrorResponse(w, http.StatusNotFound, err.Error())
		return
	}

	// Abrir archivo
	file, err := os.Open(filePath)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, "No se pudo abrir el archivo")
		return
	}
	defer file.Close()

	// Obtener información del archivo
	stat, err := file.Stat()
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, "No se pudo obtener info del archivo")
		return
	}

	fileSize := stat.Size()

	// Detectar Content-Type basado en extensión
	contentType := getContentType(filePath)
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Accept-Ranges", "bytes")

	// Manejar Range requests (para seek/reproducir desde posición)
	rangeHeader := r.Header.Get("Range")
	if rangeHeader != "" {
		h.handleRangeRequest(w, r, file, fileSize, rangeHeader)
		return
	}

	// Respuesta completa
	w.Header().Set("Content-Length", strconv.FormatInt(fileSize, 10))

	if h.config.Log != nil {
		h.config.Log.Comentario("INFO", fmt.Sprintf("Stream completo: %s", id))
	}

	if r.Method == http.MethodHead {
		return
	}

	io.Copy(w, file)
}

// handleRangeRequest maneja peticiones con header Range (para seek)
func (h *StreamHandler) handleRangeRequest(w http.ResponseWriter, r *http.Request, file *os.File, fileSize int64, rangeHeader string) {
	// Parsear Range header: "bytes=start-end"
	rangeHeader = strings.TrimPrefix(rangeHeader, "bytes=")
	parts := strings.Split(rangeHeader, "-")

	var start, end int64
	var err error

	if parts[0] != "" {
		start, err = strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			ErrorResponse(w, http.StatusRequestedRangeNotSatisfiable, "Rango inválido")
			return
		}
	}

	if len(parts) > 1 && parts[1] != "" {
		end, err = strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			ErrorResponse(w, http.StatusRequestedRangeNotSatisfiable, "Rango inválido")
			return
		}
	} else {
		end = fileSize - 1
	}

	if start > end || start >= fileSize {
		ErrorResponse(w, http.StatusRequestedRangeNotSatisfiable, "Rango inválido")
		return
	}

	length := end - start + 1

	// Seek al inicio del rango
	if _, err := file.Seek(start, io.SeekStart); err != nil {
		ErrorResponse(w, http.StatusInternalServerError, "Error en seek")
		return
	}

	// Headers de respuesta parcial
	w.Header().Set("Content-Length", strconv.FormatInt(length, 10))
	w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, fileSize))
	w.WriteHeader(http.StatusPartialContent)

	if h.config.Log != nil {
		h.config.Log.Comentario("INFO", fmt.Sprintf("Stream parcial: bytes %d-%d", start, end))
	}

	if r.Method == http.MethodHead {
		return
	}

	io.CopyN(w, file, length)
}

// getContentType retorna el MIME type basado en la extensión
func getContentType(filename string) string {
	ext := strings.ToLower(path.Ext(filename))
	switch ext {
	case ".mp3":
		return "audio/mpeg"
	case ".flac":
		return "audio/flac"
	case ".wav":
		return "audio/wav"
	case ".ogg":
		return "audio/ogg"
	case ".m4a":
		return "audio/mp4"
	case ".aac":
		return "audio/aac"
	case ".opus":
		return "audio/opus"
	default:
		return "application/octet-stream"
	}
}

// extractIP extrae la IP de RemoteAddr (quita el puerto)
func extractIP(remoteAddr string) string {
	if idx := strings.LastIndex(remoteAddr, ":"); idx != -1 {
		return remoteAddr[:idx]
	}
	return remoteAddr
}
