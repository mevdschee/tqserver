package fastcgi

import (
	"bytes"
	"encoding/binary"
	"errors"
)

var (
	ErrInvalidParamLength = errors.New("invalid parameter length")
)

// EncodeParam encodes a single name-value pair according to FastCGI spec
func EncodeParam(name, value string) []byte {
	nameLen := len(name)
	valueLen := len(value)

	var buf bytes.Buffer

	// Encode name length
	if nameLen < 128 {
		buf.WriteByte(byte(nameLen))
	} else {
		b := make([]byte, 4)
		binary.BigEndian.PutUint32(b, uint32(nameLen)|0x80000000)
		buf.Write(b)
	}

	// Encode value length
	if valueLen < 128 {
		buf.WriteByte(byte(valueLen))
	} else {
		b := make([]byte, 4)
		binary.BigEndian.PutUint32(b, uint32(valueLen)|0x80000000)
		buf.Write(b)
	}

	// Write name and value
	buf.WriteString(name)
	buf.WriteString(value)

	return buf.Bytes()
}

// EncodeParams encodes multiple name-value pairs into FastCGI params format
func EncodeParams(params map[string]string) []byte {
	var buf bytes.Buffer

	for name, value := range params {
		buf.Write(EncodeParam(name, value))
	}

	return buf.Bytes()
}

// DecodeParams decodes FastCGI params from bytes into a map
func DecodeParams(data []byte) (map[string]string, error) {
	params := make(map[string]string)

	pos := 0
	for pos < len(data) {
		// Decode name length
		if pos >= len(data) {
			return nil, ErrInvalidParamLength
		}

		nameLen, n := decodeLength(data[pos:])
		if n == 0 {
			return nil, ErrInvalidParamLength
		}
		pos += n

		// Decode value length
		if pos >= len(data) {
			return nil, ErrInvalidParamLength
		}

		valueLen, n := decodeLength(data[pos:])
		if n == 0 {
			return nil, ErrInvalidParamLength
		}
		pos += n

		// Read name
		if pos+nameLen > len(data) {
			return nil, ErrInvalidParamLength
		}
		name := string(data[pos : pos+nameLen])
		pos += nameLen

		// Read value
		if pos+valueLen > len(data) {
			return nil, ErrInvalidParamLength
		}
		value := string(data[pos : pos+valueLen])
		pos += valueLen

		params[name] = value
	}

	return params, nil
}

// decodeLength decodes a length field (1 or 4 bytes) and returns the length and bytes consumed
func decodeLength(data []byte) (length int, bytesRead int) {
	if len(data) == 0 {
		return 0, 0
	}

	if data[0] < 128 {
		return int(data[0]), 1
	}

	if len(data) < 4 {
		return 0, 0
	}

	length = int(binary.BigEndian.Uint32(data[0:4]) & 0x7fffffff)
	return length, 4
}
