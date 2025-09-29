# conversion-service

Microservicio en Go (Gin) para generar descargas de vídeos y subtítulos de Picta.

```
GET /download/{id}/{quality}        # Vídeo (presigned 48h)
GET /download/{id}/subtitle         # Subtítulos SRT
GET /buckets                        # Listado de buckets en el minio s3
```

## Instalación
1. Clonar el repositorio:
   ```bash
   git clone -b master https://github.com/dario61k/conversion_service.git
   ```
2. Entrar en el directorio del proyecto:
   ```bash
    cd conversion_service
    ```
3. Instalar las dependencias:
    ```bash
   go mod download
   ```
4. Configurar las variables de entorno necesarias (renombrar "template.env" a solo ".env"):
   ```bash
    export PG_DSN="user=postgres password=postgres host=localhost port=5432 database=db_name sslmode=disable"
    export MINIO_ENDPOINT=s3.example.com
    export MINIO_ACCESS_KEY=your_access_key
    export MINIO_SECRET_KEY=your_secret_key
    export HTTP_PORT=8080
    export DOWNLOADS_BUCKET=descargas
    export VIDEOS_BUCKET=videos
    export FFMPEG_PATH=ffmpeg
    export MINIO_USE_SSL=true
    ```
5. Ejecutar el proyecto
   ```bash
   go run main.go
   ```

* Cron diario elimina descargas >30 días.
* PostgreSQL + MinIO + Gin.