# conversion-service

Microservicio en Go para solicitar, generar y entregar descargas de videos y subtítulos bajo demanda.  
El proyecto usa:

- **PostgreSQL** para metadatos, jobs y control de estado.
- **MinIO/S3** para almacenamiento de archivos de entrada y salida.
- **RabbitMQ** para encolar trabajos de generación.
- **FFmpeg** para realizar el muxing de audio y video.
- **Gin** como framework HTTP.

Además, el workspace incluye un servicio auxiliar de simulación de carga:

- [`aux_service/main.go`](aux_service/main.go) simula usuarios concurrentes y un servidor fake de autenticación.
- [`watch_minio.sh`](watch_minio.sh) monitorea el uso del bucket de descargas.

---

## Estructura del workspace

- `conversion_service/`  
  Servicio principal.
  - `cmd/server/` servidor HTTP.
  - `cmd/worker/` proceso de workers.
  - `internal/` lógica interna: configuración, base de datos, cron, handlers, middlewares, queue, storage y servicios.
  - `.env` configuración local.
  - `template.env` plantilla de variables de entorno.

- `aux_service/`  
  Simulador de usuarios y servidor de autenticación falso para pruebas de carga.

- `watch_minio.sh`  
  Script simple para observar el crecimiento del bucket de descargas.

- `.http`  
  Petición de prueba para usar desde VS Code.

---

## Flujo funcional

### 1) Solicitud de recurso
El cliente llama:

```http
GET /download/{id}/{quality}
```

Si el archivo **ya existe** en MinIO:

- la API responde `200 OK`
- devuelve una URL presigned válida para descarga inmediata

Si el archivo **no existe**:

- la API crea o reutiliza un job
- responde `202 Accepted`
- devuelve `job_id`

---

### 2) Consulta del estado
El cliente consulta:

```http
GET /jobs/{job_id}
```

Este endpoint permite seguimiento tipo **long-polling**:

- si el job sigue en proceso, la API devuelve el estado actual
- si el job termina en `completed`, se entrega `download_url`
- si el job termina en `failed`, se devuelve el error asociado

---

### 3) Procesamiento en background
Cuando un job está activo:

- un worker consume el mensaje desde RabbitMQ
- localiza los flujos necesarios en MinIO
- ejecuta FFmpeg para realizar el muxing
- sube el archivo final al bucket de descargas
- actualiza el estado del job en PostgreSQL

---

### 4) Limpieza automática
Un cron job elimina recursos expirados según la política de retención configurada por `TTL`.

---

## Endpoints principales

### Descarga de video
```http
GET /download/{id}/{quality}
```

### Estado de job
```http
GET /jobs/{job_id}
```

### Descarga de subtítulos
```http
GET /download/{id}/subtitle/{lang}
```

### Listado de buckets
```http
GET /buckets
```

---

## Calidades válidas

Las calidades soportadas actualmente son:

```text
low
medium
high
pro
```

---

## Requisitos

- Go **1.24.3** o compatible
- PostgreSQL
- MinIO o un servicio S3 compatible
- RabbitMQ
- FFmpeg

---

## Configuración

La configuración local se toma desde `.env`.  
Como base se puede usar [`conversion_service/template.env`](conversion_service/template.env).

### Variables relevantes

```env
PG_DSN="user=postgres password=postgres host=localhost port=5432 database=db_name sslmode=disable"
MINIO_ENDPOINT=localhost:9000
MINIO_ACCESS_KEY=minio
MINIO_SECRET_KEY=miniopass
MINIO_USE_SSL=false
HTTP_PORT=8080
DOWNLOADS_BUCKET=descargas
VIDEOS_BUCKET=videos
FFMPEG_PATH=/usr/bin/ffmpeg
DEBUG=true
TTL=5
AUTH_ENDPOINT=http://localhost:3000/api/auth/check
AMQP_URL=amqp://localhost:5672/
AMQP_EXCHANGE=conversion.direct
AMQP_QUEUE_BUILD=video.build.request
AMQP_QUEUE_RETRY=video.build.retry
AMQP_QUEUE_DLQ=video.build.dlq
AMQP_PREFETCH=5
AMQP_RETRY_TTL_MS=15000
AMQP_MAX_ATTEMPTS=5
LONG_POLL_TIMEOUT_SEC=30
LONG_POLL_INTERVAL_MS=1000
WORKER_COUNT=4
```

### Notas sobre configuración

- `AUTH_ENDPOINT` debe apuntar al servidor de autenticación real o al fake auth server del [`aux_service`](aux_service/main.go).
- `TTL` define el tiempo de retención de recursos expirados.
- `DOWNLOADS_BUCKET` y `VIDEOS_BUCKET` deben coincidir con los buckets existentes en MinIO.
- `WORKER_COUNT` define cuántos workers levantará el proceso de fondo.

---

## Ejecución

### 1) Levantar el servicio principal
Desde `conversion_service/`:

```bash
go run ./cmd/server
```

### 2) Levantar el simulador de carga
Desde `aux_service/`:

```bash
go run .
```

Este simulador:

- expone un fake auth server en `:3000`
- genera requests concurrentes contra `http://localhost:8080`
- hace polling a `/jobs/{job_id}` cuando la respuesta inicial es `202`

---

## Simulación de pruebas

El archivo [`aux_service/main.go`](aux_service/main.go) sirve para:

- simular múltiples usuarios concurrentes
- medir respuestas `200` y `202`
- observar tiempos de latencia
- validar el flujo de jobs
- generar carga repetitiva sobre el API

El archivo `.http` también permite validar manualmente la API desde VS Code:

```http
GET http://localhost:8080/download/2/low
```

---

## Monitoreo de almacenamiento

El script [`watch_minio.sh`](watch_minio.sh) imprime periódicamente el uso del bucket de descargas:

```bash
./watch_minio.sh
```

---

## Estado funcional esperado

El comportamiento esperado del sistema es:

- **200** cuando el recurso ya existe y se puede entregar de inmediato.
- **202** cuando la solicitud fue aceptada y quedó en cola o en proceso.
- **Estado `completed`** cuando el job terminó correctamente.
- **Estado `failed`** cuando el worker no pudo generar el recurso.

---

## Observaciones

- El proyecto está pensado para trabajar con solicitudes concurrentes.
- La deduplicación de jobs evita crear trabajos repetidos para el mismo recurso y calidad.
- La entrega final se hace mediante URL presigned, sin exponer la ruta interna de MinIO.
- La limpieza automática evita acumulación de recursos expirados.

---


## Archivos útiles

- [`conversion_service/template.env`](conversion_service/template.env)
- [`aux_service/main.go`](aux_service/main.go)
- [`watch_minio.sh`](watch_minio.sh)
- [`.http`](.http)