package services

import "errors"

var ErrNotFound = errors.New("contenido no encontrado")
var ErrInvalidRequest = errors.New("solicitud inválida")
var ErrForbidden = errors.New("acceso denegado")
