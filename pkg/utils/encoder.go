package utils

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"io/ioutil"
	"strings"
)

// EncodeBase64URL codifica datos en Base64 URL-safe
func EncodeBase64URL(data []byte) string {
	return base64.URLEncoding.EncodeToString(data)
}

// DecodeBase64URL decodifica datos en Base64 URL-safe
func DecodeBase64URL(encoded string) ([]byte, error) {
	// Manejar padding si es necesario
	if m := len(encoded) % 4; m != 0 {
		encoded += strings.Repeat("=", 4-m)
	}

	return base64.URLEncoding.DecodeString(encoded)
}

// JSONMarshal codifica una estructura en JSON
// JSONMarshal codifica una estructura en JSON
func JSONMarshal(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

// JSONUnmarshal decodifica JSON en una estructura
func JSONUnmarshal(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}

// CompressData comprime datos usando gzip
func CompressData(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	gzw := gzip.NewWriter(&buf)

	_, err := gzw.Write(data)
	if err != nil {
		return nil, err
	}

	if err := gzw.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// DecompressData descomprime datos gzip
func DecompressData(compressedData []byte) ([]byte, error) {
	buf := bytes.NewBuffer(compressedData)
	gzr, err := gzip.NewReader(buf)
	if err != nil {
		return nil, err
	}
	defer gzr.Close()

	return ioutil.ReadAll(gzr)
}
