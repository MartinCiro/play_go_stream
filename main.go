package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go_stream/controller"
)

func main() {
	fmt.Println("=======================================================")
	fmt.Println("🎵 go_stream - Servidor de Música")
	fmt.Println("=======================================================")

	// 1️⃣ Configuración
	cfg := controller.NewConfig()

	// 2️⃣ Logger
	log := controller.NewLogger(cfg.LogLevel, cfg.LogDir)
	defer log.Close()
	log.InicioProceso("go_stream")

	// 3️⃣ Servidor
	srv := controller.NewServer(cfg, log)

	// 4️⃣ Shutdown graceful
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Run()
	}()

	// 5️⃣ Esperar señal o error
	select {
	case sig := <-sigCh:
		log.Info("Señal recibida: %v", sig)
	case err := <-errCh:
		if err != nil {
			log.Error("Servidor terminó con error: %v", err)
		}
	}

	// 6️⃣ Apagar con timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Error("Error durante shutdown: %v", err)
	}

	log.FinProceso("go_stream")
	fmt.Println("🛑 go_stream detenido")
}
