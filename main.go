package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"go_stream/controller"
)

func main() {
	fmt.Println("==================================================")
	fmt.Println("🎵 Go Stream - Servidor de Música")
	fmt.Println("==================================================")

	// 1️⃣ Instanciar configuración
	config := controller.NewConfig()

	// 2️⃣ Instanciar servicios
	musicService := controller.NewMusicService(config)
	streamHandler := controller.NewStreamHandler(config, musicService)

	config.Log.InicioProceso("Go Stream")
	config.Log.Comentario("SUCCESS", "Servicios inicializados")

	// 3️⃣ Cargar biblioteca de música
	if err := musicService.LoadLibrary(); err != nil {
		config.Log.Error(fmt.Sprintf("Error cargando biblioteca: %v", err), "MusicService")
		log.Fatalf("❌ Error: %v", err)
	}

	// 4️⃣ Información inicial
	fmt.Printf("🎵 Biblioteca: %s\n", config.MusicPath)
	fmt.Printf("📚 Canciones:  %d\n", musicService.Count())
	fmt.Printf("🌐 Servidor:   http://localhost:%s\n", config.Port)
	fmt.Println("==================================================")

	// 5️⃣ Iniciar servidor HTTP en goroutine
	go func() {
		if err := streamHandler.Start(); err != nil {
			config.Log.Error(fmt.Sprintf("Error iniciando servidor: %v", err), "HTTP")
			log.Fatalf("❌ Error: %v", err)
		}
	}()

	// 6️⃣ Shutdown graceful
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 7️⃣ Esperar señal
	<-sigChan
	config.Log.Comentario("INFO", "Recibida señal de terminación")
	config.Log.FinProceso("Go Stream")
	fmt.Println("\n🛑 Servidor detenido")
}
