# 🎵 PlayGo Stream - Servidor Local de Música

> **Servidor de streaming de audio ligero y privado** construido en Go. Sirve tu biblioteca de música local (MP3, FLAC, OGG, WAV) a través de una API REST, soportando metadatos, streaming por chunks y control de acceso.

A diferencia de un bot que busca en internet, **PlayGo Stream** está diseñado para ser el backend de tu propia colección de música. Funciona como un "mini Spotify" personal que corre en tu máquina, Raspberry Pi o servidor, sin depender de la nube ni enviar tus datos a terceros.

---

## 🚀 Características Principales

- 🎧 **Streaming Real con Range Requests**: Soporta cabeceras HTTP `Range`, permitiendo a los reproductores clientes pausar, reanudar y avanzar (seek) en la canción sin descargar el archivo completo.
- 🏷️ **Extracción Automática de Metadatos**: Lee etiquetas ID3 y metadatos (Título, Artista, Álbum, Año, Duración) directamente de los archivos usando `github.com/dhowden/tag`.
- 🛡️ **Control de Acceso por IP**: Configuración de lista blanca (whitelist) de IPs permitidas para proteger tu biblioteca en redes locales.
- ⚡ **Arquitectura por Capas (MVC Simplificado)**: Separación clara entre configuración, lógica de negocio (servicio), controladores HTTP y modelos de datos.
- 🛑 **Apagado Elegante (Graceful Shutdown)**: Manejo correcto de señales `SIGINT` y `SIGTERM` para cerrar conexiones y liberar recursos de forma segura.
- 📝 **Logging Estructurado**: Sistema de registro de eventos con rotación y niveles de severidad para monitoreo.

---

## 🏗️ Arquitectura del Sistema

El proyecto sigue un patrón de diseño por capas, garantizando que la lógica de negocio esté desacoplada de la capa de transporte HTTP.

```mermaid
graph TD
    Client((🎧 Cliente HTTP / Reproductor)) -->|Solicitudes GET| Handler[StreamHandler.go<br/>Controlador HTTP]
    
    subgraph "🧠 Capa de Control y Lógica"
        Handler -->|Consulta biblioteca| Service[MusicService.go<br/>Gestión de Metadatos y Archivos]
        Handler -->|Formatea respuesta| Response[Response.go<br/>JSON Estándar]
        Service -->|Define estructura| Track[track.go<br/>Modelo de Datos]
    end
    
    subgraph "💾 Capa de Persistencia"
        Service -->|Escanea y lee| FS[(Sistema de Archivos<br/>Carpeta de Música Local)]
        FS -->|Devuelve bytes| Handler
    end
    
    subgraph "⚙️ Configuración"
        Config[Config.go<br/>Variables de Entorno] -.-> Handler
        Config -.-> Service
    end

    style Client fill:#e0f2fe,stroke:#0284c7,stroke-width:2px
    style Service fill:#fff3e0,stroke:#d97706,stroke-width:2px
    style FS fill:#dcfce7,stroke:#16a34a,stroke-width:2px
```

---

## 📂 Estructura del Proyecto

```text
play_go_stream/
├── main.go                    # 🎯 Punto de entrada: inicialización, carga de biblioteca y servidor HTTP
├── controller/
│   ├── Config.go              # 🔧 Carga de variables de entorno (.env) y configuración de logging
│   ├── Log.go                 # 📝 Sistema de logging con rotación y niveles de severidad
│   ├── MusicService.go        # 🎵 Escaneo de directorios, lectura de metadatos (ID3) y gestión en memoria
│   ├── StreamHandler.go       # 🌐 Controlador HTTP: maneja rutas, CORS y Range Requests (HTTP 206)
│   ├── Response.go            # 📦 Utilidades para respuestas JSON estandarizadas
│   └── track.go               # 🎼 Definición de la estructura de datos `Track` (ID, Título, Duración, Hash, etc.)
├── go.mod                     # Dependencias del módulo (Go 1.26.5)
└── go.sum                     # Checksum de dependencias
```

---

## 🔄 Flujo de Streaming (Range Requests)

Este diagrama muestra cómo el servidor maneja eficientemente las solicitudes de reproducción, permitiendo a los clientes solicitar solo fragmentos específicos del archivo de audio.

```mermaid
sequenceDiagram
    autonumber
    participant Client as 🎧 Reproductor (Cliente)
    participant Handler as 🌐 StreamHandler
    participant Service as 🎵 MusicService
    participant FS as 💾 Sistema de Archivos

    Client->>Handler: GET /stream/{id}
    Handler->>Service: Obtener ruta del archivo por ID
    Service-->>Handler: FilePath absoluto
    
    Handler->>FS: Abrir archivo y obtener FileInfo (Size)
    
    alt Cliente solicita rango (ej. Pausa/Seek)
        Handler->>Handler: Parsear header "Range: bytes=1000-2000"
        Handler->>FS: Seek a la posición 1000
        Handler-->>Client: HTTP 206 Partial Content + Content-Range
    else Cliente solicita archivo completo
        Handler-->>Client: HTTP 200 OK + Content-Length
    end
    
    loop Streaming de datos
        Handler->>FS: Leer chunk de bytes (ej. 32KB)
        FS-->>Handler: Bytes de audio
        Handler-->>Client: Enviar chunk por la conexión HTTP
    end
    
    Client->>Handler: Cerrar conexión
    Handler->>FS: Cerrar descriptor de archivo
```

---

## 📋 Requisitos Previos

- **Go 1.21+** (El proyecto está configurado para 1.26.5)
- Una carpeta con archivos de audio (`.mp3`, `.flac`, `.ogg`, `.wav`)

---

## 🛠️ Instalación y Configuración

### 1. Clonar y preparar el entorno
```bash
git clone https://github.com/MartinCiro/play_go_stream.git
cd play_go_stream
```

### 2. Configurar variables de entorno
Crea un archivo `.env` en la raíz del proyecto basado en la configuración esperada:
```env
# Puerto en el que escuchará el servidor
PORT=8080

# Ruta absoluta a tu biblioteca de música
MUSIC_PATH=/ruta/a/tu/carpeta/de/musica

# IPs permitidas para acceder al servidor (separadas por coma, o "*" para todas)
ALLOWED_IPS=127.0.0.1,192.168.1.*
```

### 3. Instalar dependencias y ejecutar
```bash
# Descargar dependencias (dhowden/tag, godotenv, etc.)
go mod tidy

# Ejecutar en modo desarrollo
go run main.go
```

### 4. Compilar para producción
```bash
go build -o playgo-stream main.go
./playgo-stream
```

---

## 📚 Endpoints de la API

| Método | Ruta | Descripción |
| :--- | :--- | :--- |
| `GET` | `/api/status` | Verifica que el servidor esté corriendo y muestra el conteo de canciones. |
| `GET` | `/api/songs` | Devuelve una lista JSON con los metadatos de todas las canciones indexadas. |
| `GET` | `/api/song/{id}` | Devuelve los metadatos detallados de una canción específica. |
| `GET` | `/stream/{id}` | **Streaming de audio**. Soporta cabeceras `Range` para reproducción eficiente (HTTP 200 / 206). |

---

## 🔒 Seguridad y Privacidad

- **100% Local**: No se envía ninguna información a servidores externos. Todo el procesamiento de metadatos y streaming ocurre en tu máquina.
- **Sin Base de Datos Externa**: La biblioteca se carga en memoria al iniciar, lo que garantiza velocidad y cero dependencias de servicios como SQLite o PostgreSQL para la operación básica.
- **Filtrado de IP**: El middleware de seguridad rechaza automáticamente solicitudes de IPs no autorizadas, protegiendo tu red local.

---

## 🤝 Contribuciones

Las contribuciones son bienvenidas. Si deseas mejorar PlayGo Stream:
1. Haz un Fork del repositorio.
2. Crea una rama para tu funcionalidad (`git checkout -b feature/AmazingFeature`).
3. Realiza tus cambios y haz commit (`git commit -m 'Add some AmazingFeature'`).
4. Haz Push a la rama y abre un Pull Request.

---

## 👤 Autor

**Martin Ciro**  
[![GitHub](https://img.shields.io/badge/GitHub-MartinCiro-181717?style=flat&logo=github)](https://github.com/MartinCiro)

---
*Desarrollado con Go, priorizando la eficiencia, la privacidad y el control total sobre tu biblioteca de música.*