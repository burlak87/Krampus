package compression

import (
	"bytes"

	"github.com/andybalholm/brotli"
)

func CompressBrotli(data []byte) ([]byte, error) {
	var buf bytes.Buffer

	writer := brotli.NewWriter(&buf)

	_, err := writer.Write(data)
	if err != nil {
		return nil, err
	}

	err = writer.Close()
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
