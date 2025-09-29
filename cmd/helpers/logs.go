package helpers

import (
	"fmt"
	"io"
	"log"
	"os"
)

var logsPath string = "./logs"

// BuildLogs regresa un writer que envía la salida normal a stdout y a gin.log.
func BuildLogs() io.Writer {
	f, err := os.OpenFile(fmt.Sprintf("%s/gin.log", logsPath), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		log.Fatalf("abrir gin.log: %v", err)
	}
	return io.MultiWriter(os.Stdout, f)
}

// BuildErrorLogs regresa un writer que envía los errores a stderr y a gin.error.log.
func BuildErrorLogs() io.Writer {
	f, err := os.OpenFile(fmt.Sprintf("%s/gin.error.log", logsPath), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		log.Fatalf("abrir gin.error.log: %v", err)
	}
	return io.MultiWriter(os.Stderr, f)
}
