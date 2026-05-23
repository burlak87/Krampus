package compression

import "github.com/klauspost/compress/zstd"

func CompressZSTD(data []byte) ([]byte, error) {
	encoder, err := zstd.NewWriter(nil)
	if err != nil {
		return nil, err
	}

	return encoder.EncodeAll(data, make([]byte, 0, len(data))), nil
}
