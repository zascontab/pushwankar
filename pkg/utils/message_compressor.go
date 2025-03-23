package utils

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"io"
)

// MessageCompressor proporciona funciones para comprimir/descomprimir mensajes
type MessageCompressor struct {
	// Umbral en bytes a partir del cual comprimir mensajes
	threshold int
	// Nivel de compresión (1-9, donde 9 es la máxima compresión)
	level int
}

// NewMessageCompressor crea una nueva instancia de MessageCompressor
func NewMessageCompressor(threshold, level int) *MessageCompressor {
	// Asegurar que el nivel esté en el rango válido
	if level < gzip.NoCompression || level > gzip.BestCompression {
		level = gzip.DefaultCompression
	}

	return &MessageCompressor{
		threshold: threshold,
		level:     level,
	}
}

// CompressJSON comprime un objeto JSON y lo codifica en base64 si supera el umbral
func (c *MessageCompressor) CompressJSON(data interface{}) (string, bool, error) {
	// Convertir a JSON
	jsonData, err := json.Marshal(data)
	if err != nil {
		return "", false, err
	}

	// Si el tamaño está por debajo del umbral, no comprimir
	if len(jsonData) < c.threshold {
		return string(jsonData), false, nil
	}

	// Comprimir los datos
	var buf bytes.Buffer
	gzWriter, err := gzip.NewWriterLevel(&buf, c.level)
	if err != nil {
		return "", false, err
	}

	_, err = gzWriter.Write(jsonData)
	if err != nil {
		return "", false, err
	}

	err = gzWriter.Close()
	if err != nil {
		return "", false, err
	}

	// Codificar en base64
	encoded := base64.StdEncoding.EncodeToString(buf.Bytes())
	return encoded, true, nil
}

// DecompressJSON descomprime un mensaje JSON en base64
func (c *MessageCompressor) DecompressJSON(data string, compressed bool, target interface{}) error {
	var jsonData []byte
	//var err error

	if compressed {
		// Decodificar base64
		decoded, err := base64.StdEncoding.DecodeString(data)
		if err != nil {
			return err
		}

		// Descomprimir gzip
		gzReader, err := gzip.NewReader(bytes.NewReader(decoded))
		if err != nil {
			return err
		}
		defer gzReader.Close()

		jsonData, err = io.ReadAll(gzReader)
		if err != nil {
			return err
		}
	} else {
		jsonData = []byte(data)
	}

	// Decodificar JSON
	return json.Unmarshal(jsonData, target)
}

// CompressMessage comprime un mensaje WebSocket
// Esta función toma un mensaje y devuelve una versión comprimida si es necesario
func (c *MessageCompressor) CompressMessage(message []byte) ([]byte, bool) {
	// Si el mensaje es menor que el umbral, no comprimir
	if len(message) < c.threshold {
		return message, false
	}

	// Comprimir con gzip
	var buf bytes.Buffer
	gzWriter, err := gzip.NewWriterLevel(&buf, c.level)
	if err != nil {
		return message, false
	}

	_, err = gzWriter.Write(message)
	if err != nil {
		return message, false
	}

	err = gzWriter.Close()
	if err != nil {
		return message, false
	}

	compressed := buf.Bytes()

	// Comprobar si la compresión redujo realmente el tamaño
	if len(compressed) >= len(message) {
		return message, false
	}

	// Crear mensaje con cabecera de compresión
	result := make([]byte, len(compressed)+1)
	result[0] = 1 // Flag que indica que el mensaje está comprimido
	copy(result[1:], compressed)

	return result, true
}

// DecompressMessage descomprime un mensaje WebSocket si es necesario
func (c *MessageCompressor) DecompressMessage(message []byte) ([]byte, error) {
	// Verificar si el mensaje está comprimido
	if len(message) == 0 || message[0] != 1 {
		return message, nil
	}

	// Descomprimir
	gzReader, err := gzip.NewReader(bytes.NewReader(message[1:]))
	if err != nil {
		return nil, err
	}
	defer gzReader.Close()

	decompressed, err := io.ReadAll(gzReader)
	if err != nil {
		return nil, err
	}

	return decompressed, nil
}
