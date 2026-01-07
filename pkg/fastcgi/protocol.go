package fastcgi

import (
"encoding/binary"
"fmt"
)

// FastCGI protocol constants
const (
// Version is the FastCGI protocol version
Version1 uint8 = 1

// Record types
TypeBeginRequest    uint8 = 1
TypeAbortRequest    uint8 = 2
TypeEndRequest      uint8 = 3
TypeParams          uint8 = 4
TypeStdin           uint8 = 5
TypeStdout          uint8 = 6
TypeStderr          uint8 = 7
TypeData            uint8 = 8
TypeGetValues       uint8 = 9
TypeGetValuesResult uint8 = 10
TypeUnknownType     uint8 = 11

// Roles
RoleResponder  uint16 = 1
RoleAuthorizer uint16 = 2
RoleFilter     uint16 = 3

// Flags
FlagKeepConn uint8 = 1

// Protocol status codes
StatusRequestComplete uint32 = 0
StatusCantMultiplex   uint32 = 1
StatusOverloaded      uint32 = 2
StatusUnknownRole     uint32 = 3

// Header size
HeaderSize = 8

// Max record content length
MaxContentLength = 65535
)

// Header represents a FastCGI record header
type Header struct {
Version       uint8
Type          uint8
RequestID     uint16
ContentLength uint16
PaddingLength uint8
Reserved      uint8
}

// EncodeHeader encodes a header into bytes
func (h *Header) Encode() []byte {
buf := make([]byte, HeaderSize)
buf[0] = h.Version
buf[1] = h.Type
binary.BigEndian.PutUint16(buf[2:4], h.RequestID)
binary.BigEndian.PutUint16(buf[4:6], h.ContentLength)
buf[6] = h.PaddingLength
buf[7] = h.Reserved
return buf
}

// DecodeHeader decodes a header from bytes
func DecodeHeader(data []byte) (*Header, error) {
if len(data) < HeaderSize {
return nil, fmt.Errorf("invalid header length: %d", len(data))
}
return &Header{
Version:       data[0],
Type:          data[1],
RequestID:     binary.BigEndian.Uint16(data[2:4]),
ContentLength: binary.BigEndian.Uint16(data[4:6]),
PaddingLength: data[6],
Reserved:      data[7],
}, nil
}

// BeginRequestBody represents the body of a BeginRequest record
type BeginRequestBody struct {
Role     uint16
Flags    uint8
Reserved [5]uint8
}

// EncodeBeginRequestBody encodes a BeginRequest body into bytes
func (b *BeginRequestBody) Encode() []byte {
buf := make([]byte, 8)
binary.BigEndian.PutUint16(buf[0:2], b.Role)
buf[2] = b.Flags
copy(buf[3:8], b.Reserved[:])
return buf
}

// DecodeBeginRequestBody decodes a BeginRequest body from bytes
func DecodeBeginRequestBody(data []byte) (*BeginRequestBody, error) {
if len(data) < 8 {
return nil, fmt.Errorf("invalid BeginRequest body length: %d", len(data))
}
body := &BeginRequestBody{
Role:  binary.BigEndian.Uint16(data[0:2]),
Flags: data[2],
}
copy(body.Reserved[:], data[3:8])
return body, nil
}

// EndRequestBody represents the body of an EndRequest record
type EndRequestBody struct {
AppStatus      uint32
ProtocolStatus uint8
Reserved       [3]uint8
}

// EncodeEndRequestBody encodes an EndRequest body into bytes
func (e *EndRequestBody) Encode() []byte {
buf := make([]byte, 8)
binary.BigEndian.PutUint32(buf[0:4], e.AppStatus)
buf[4] = e.ProtocolStatus
copy(buf[5:8], e.Reserved[:])
return buf
}

// DecodeEndRequestBody decodes an EndRequest body from bytes
func DecodeEndRequestBody(data []byte) (*EndRequestBody, error) {
if len(data) < 8 {
return nil, fmt.Errorf("invalid EndRequest body length: %d", len(data))
}
body := &EndRequestBody{
AppStatus:      binary.BigEndian.Uint32(data[0:4]),
ProtocolStatus: data[4],
}
copy(body.Reserved[:], data[5:8])
return body, nil
}

// Record represents a complete FastCGI record
type Record struct {
Header  *Header
Content []byte
Padding []byte
}

// NewRecord creates a new record with the given type, request ID, and content
func NewRecord(typ uint8, requestID uint16, content []byte) *Record {
contentLen := len(content)
paddingLen := (8 - (contentLen % 8)) % 8

return &Record{
Header: &Header{
Version:       Version1,
Type:          typ,
RequestID:     requestID,
ContentLength: uint16(contentLen),
PaddingLength: uint8(paddingLen),
},
Content: content,
Padding: make([]byte, paddingLen),
}
}

// Encode encodes a record into bytes
func (r *Record) Encode() []byte {
headerBytes := r.Header.Encode()
result := make([]byte, 0, len(headerBytes)+len(r.Content)+len(r.Padding))
result = append(result, headerBytes...)
result = append(result, r.Content...)
result = append(result, r.Padding...)
return result
}

// DecodeRecord decodes a record from bytes
func DecodeRecord(data []byte) (*Record, int, error) {
if len(data) < HeaderSize {
return nil, 0, fmt.Errorf("insufficient data for header")
}

header, err := DecodeHeader(data[:HeaderSize])
if err != nil {
return nil, 0, err
}

totalLen := HeaderSize + int(header.ContentLength) + int(header.PaddingLength)
if len(data) < totalLen {
return nil, 0, fmt.Errorf("insufficient data for record: need %d, have %d", totalLen, len(data))
}

contentStart := HeaderSize
contentEnd := contentStart + int(header.ContentLength)
paddingEnd := contentEnd + int(header.PaddingLength)

return &Record{
Header:  header,
Content: data[contentStart:contentEnd],
Padding: data[contentEnd:paddingEnd],
}, totalLen, nil
}
