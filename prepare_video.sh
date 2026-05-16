#!/bin/bash

set -e

if [ "$#" -ne 2 ]; then
    echo "Uso: $0 <ruta_video> <nombre>"
    echo "Ejemplo: $0 /home/dario/videos/dispatch_trailer.mp4 dispatch"
    exit 1
fi

INPUT="$1"
NAME="$2"

if [ ! -f "$INPUT" ]; then
    echo "Error: no se encontró el archivo $INPUT"
    exit 1
fi

echo "Procesando '$INPUT' como '$NAME'..."

mkdir -p "$NAME"/{low,medium,high,pro}

echo "[1/8] Audio low..."
ffmpeg -i "$INPUT" -vn -acodec aac -b:a 96k "$NAME/low/audio.m4a" -y -loglevel error

echo "[2/8] Video low (144p)..."
ffmpeg -i "$INPUT" -an -vf scale=-2:144 -c:v libx264 -crf 28 -preset fast "$NAME/low/output_144p.mp4" -y -loglevel error

echo "[3/8] Audio medium..."
ffmpeg -i "$INPUT" -vn -acodec aac -b:a 128k "$NAME/medium/audio.m4a" -y -loglevel error

echo "[4/8] Video medium (360p)..."
ffmpeg -i "$INPUT" -an -vf scale=-2:360 -c:v libx264 -crf 26 -preset fast "$NAME/medium/output_360p.mp4" -y -loglevel error

echo "[5/8] Audio high..."
ffmpeg -i "$INPUT" -vn -acodec aac -b:a 192k "$NAME/high/audio.m4a" -y -loglevel error

echo "[6/8] Video high (720p)..."
ffmpeg -i "$INPUT" -an -vf scale=-2:720 -c:v libx264 -crf 23 -preset fast "$NAME/high/output_720p.mp4" -y -loglevel error

echo "[7/8] Audio pro..."
ffmpeg -i "$INPUT" -vn -acodec aac -b:a 256k "$NAME/pro/audio.m4a" -y -loglevel error

echo "[8/8] Video pro (1080p)..."
ffmpeg -i "$INPUT" -an -vf scale=-2:1080 -c:v libx264 -crf 20 -preset fast "$NAME/pro/output_1080p.mp4" -y -loglevel error

echo ""
echo "Estructura generada:"
find "$NAME" -type f | sort

echo ""
echo "Tamaños:"
du -sh "$NAME"/*/
