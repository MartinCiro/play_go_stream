package controller

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"
)

// Server encapsula el servidor HTTP con shutdown graceful
type Server struct {
	cfg  *Config
	log  *Logger
	http *http.Server
	lib  *Library
}

// NewServer construye el servidor con todas sus dependencias
func NewServer(cfg *Config, log *Logger) *Server {
	lib := NewLibrary(cfg, log)
	streamer := NewStreamer(lib, log)
	handlers := NewHandlers(lib, streamer, log)

	mux := http.NewServeMux()

	// Rutas (Go 1.22+ con método + path params)
	mux.HandleFunc("GET /health", handlers.Health)
	mux.HandleFunc("GET /api/tracks", handlers.ListTracks)
	mux.HandleFunc("GET /api/tracks/{id}", handlers.GetTrack)
	mux.HandleFunc("GET /api/stream/{id}", handlers.StreamTrack)
	mux.HandleFunc("POST /api/rescan", handlers.Rescan)

	// Middleware simple de logging
	loggedMux := logMiddleware(mux, log)

	srv := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.ServerHost, cfg.ServerPort),
		Handler:      loggedMux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 0, // 0 = sin límite para streaming largo
		IdleTimeout:  60 * time.Second,
	}

	return &Server{
		cfg:  cfg,
		log:  log,
		http: srv,
		lib:  lib,
	}
}

// Run inicia el servidor (bloqueante)
func (s *Server) Run() error {
	// Escaneo inicial
	if err := s.lib.Scan(); err != nil {
		s.log.Error("Escaneo inicial falló: %v", err)
		return err
	}

	s.log.Info("🎧 Servidor escuchando en http://%s", s.http.Addr)
	s.log.Info("📚 Tracks disponibles: %d", s.lib.Count())

	err := s.http.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

// Shutdown detiene el servidor de forma limpia
func (s *Server) Shutdown(ctx context.Context) error {
	s.log.Info("Apagando servidor HTTP...")
	return s.http.Shutdown(ctx)
}

// logMiddleware registra cada request
func logMiddleware(next http.Handler, log *Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		// Wrapper para capturar status
		lrw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(lrw, r)
		log.Debug("%s %s -> %d (%s)", r.Method, r.URL.Path, lrw.status, time.Since(start))
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}
