package controller

import (
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"strconv"
	"strings"
)

// Streamer sirve contenido de audio desde disco
type Streamer struct {
	lib *Library
	log *Logger
}

// NewStreamer crea un streamer conectado a la biblioteca
func NewStreamer(lib *Library, log *Logger) *Streamer {
	return &Streamer{lib: lib, log: log}
}

// ServeHTTP escribe el archivo al response con soporte Range
func (s *Streamer) ServeHTTP(w http.ResponseWriter, r *http.Request, trackID string) {
	track, ok := s.lib.Get(trackID)
	if !ok {
		http.Error(w, "track not found", http.StatusNotFound)
		return
	}

	f, err := os.Open(track.Path)
	if err != nil {
		s.log.Error("No se pudo abrir %s: %v", track.Path, err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	contentType := s.mimeFor(track.Format)
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Accept-Ranges", "bytes")

	// Parsear header Range manualmente (para tener control total)
	rangeHeader := r.Header.Get("Range")
	if rangeHeader == "" {
		// Stream completo
		w.Header().Set("Content-Length", strconv.FormatInt(stat.Size(), 10))
		w.WriteHeader(http.StatusOK)
		io.Copy(w, f)
		return
	}

	// Formato esperado: "bytes=START-END" (END opcional)
	if !strings.HasPrefix(rangeHeader, "bytes=") {
		http.Error(w, "invalid range", http.StatusRequestedRangeNotSatisfiable)
		return
	}

	rangeSpec := strings.TrimPrefix(rangeHeader, "bytes=")
	parts := strings.Split(rangeSpec, "-")
	if len(parts) != 2 {
		http.Error(w, "invalid range", http.StatusRequestedRangeNotSatisfiable)
		return
	}

	size := stat.Size()
	var start, end int64

	if parts[0] == "" {
		// Suffix range: "-500" = últimos 500 bytes
		n, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil || n <= 0 {
			http.Error(w, "invalid range", http.StatusRequestedRangeNotSatisfiable)
			return
		}
		start = size - n
		end = size - 1
	} else {
		start, err = strconv.ParseInt(parts[0], 10, 64)
		if err != nil || start < 0 {
			http.Error(w, "invalid range", http.StatusRequestedRangeNotSatisfiable)
			return
		}
		if parts[1] == "" {
			end = size - 1
		} else {
			end, err = strconv.ParseInt(parts[1], 10, 64)
			if err != nil {
				http.Error(w, "invalid range", http.StatusRequestedRangeNotSatisfiable)
				return
			}
		}
	}

	if start > end || start >= size {
		w.Header().Set("Content-Range", fmt.Sprintf("bytes */%d", size))
		http.Error(w, "range not satisfiable", http.StatusRequestedRangeNotSatisfiable)
		return
	}
	if end >= size {
		end = size - 1
	}

	length := end - start + 1
	if _, err := f.Seek(start, io.SeekStart); err != nil {
		http.Error(w, "seek error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, size))
	w.Header().Set("Content-Length", strconv.FormatInt(length, 10))
	w.WriteHeader(http.StatusPartialContent)

	io.CopyN(w, f, length)
}

func (s *Streamer) mimeFor(ext string) string {
	switch strings.ToLower(ext) {
	case "mp3":
		return "audio/mpeg"
	case "flac":
		return "audio/flac"
	case "ogg":
		return "audio/ogg"
	case "m4a":
		return "audio/mp4"
	case "wav":
		return "audio/wav"
	default:
		if mt := mime.TypeByExtension("." + ext); mt != "" {
			return mt
		}
		return "application/octet-stream"
	}
}

// ErrTrackNotFound error exportado para handlers
var ErrTrackNotFound = errors.New("track not found")
