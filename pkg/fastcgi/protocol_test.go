package fastcgi

import (
	"log"
	"testing"
)

func TestHeaderEncodeDecode(t *testing.T) {
	tests := []struct {
		name          string
		recType       uint8
		reqID         uint16
		contentLength uint16
	}{
		{"BeginRequest", TypeBeginRequest, 1, 8},
		{"Params", TypeParams, 1, 100},
		{"Stdin", TypeStdin, 1, 0},
		{"Stdout", TypeStdout, 1, 256},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			header := &Header{
				Version:       Version1,
				Type:          tt.recType,
				RequestID:     tt.reqID,
				ContentLength: tt.contentLength,
			}

			encoded := header.Encode()

			decoded, err := DecodeHeader(encoded)
			if err != nil {
				t.Fatalf("DecodeHeader failed: %v", err)
			}

			if decoded.Version != Version1 {
				t.Errorf("Version = %d, want %d", decoded.Version, Version1)
			}
			if decoded.Type != tt.recType {
				t.Errorf("Type = %d, want %d", decoded.Type, tt.recType)
			}
			if decoded.RequestID != tt.reqID {
				t.Errorf("RequestID = %d, want %d", decoded.RequestID, tt.reqID)
			}
			if decoded.ContentLength != tt.contentLength {
				t.Errorf("ContentLength = %d, want %d", decoded.ContentLength, tt.contentLength)
			}
		})
	}
}

func TestBeginRequestBodyEncodeDecode(t *testing.T) {
	body := &BeginRequestBody{
		Role:  RoleResponder,
		Flags: FlagKeepConn,
	}

	encoded := body.Encode()

	decoded, err := DecodeBeginRequestBody(encoded)
	if err != nil {
		t.Fatalf("DecodeBeginRequestBody failed: %v", err)
	}

	if decoded.Role != RoleResponder {
		t.Errorf("Role = %d, want %d", decoded.Role, RoleResponder)
	}
	if decoded.Flags != FlagKeepConn {
		t.Errorf("Flags = %d, want %d", decoded.Flags, FlagKeepConn)
	}
}

func TestEndRequestBodyEncodeDecode(t *testing.T) {
	body := &EndRequestBody{
		AppStatus:      0,
		ProtocolStatus: uint8(StatusRequestComplete),
	}

	encoded := body.Encode()

	decoded, err := DecodeEndRequestBody(encoded)
	if err != nil {
		t.Fatalf("DecodeEndRequestBody failed: %v", err)
	}

	if decoded.AppStatus != 0 {
		t.Errorf("AppStatus = %d, want 0", decoded.AppStatus)
	}
	if decoded.ProtocolStatus != uint8(StatusRequestComplete) {
		t.Errorf("ProtocolStatus = %d, want %d", decoded.ProtocolStatus, uint8(StatusRequestComplete))
	}
}

func TestRecordEncodeDecode(t *testing.T) {
	content := []byte("Hello, FastCGI!")
	record := NewRecord(TypeStdout, 1, content)

	encoded := record.Encode()

	decoded, bytesRead, err := DecodeRecord(encoded)
	if err != nil {
		t.Fatalf("DecodeRecord failed: %v", err)
	}

	if bytesRead != len(encoded) {
		t.Errorf("bytesRead = %d, want %d", bytesRead, len(encoded))
	}

	if decoded.Header.Type != TypeStdout {
		t.Errorf("Type = %d, want %d", decoded.Header.Type, TypeStdout)
	}
	if decoded.Header.RequestID != 1 {
		t.Errorf("RequestID = %d, want 1", decoded.Header.RequestID)
	}
	if string(decoded.Content) != string(content) {
		t.Errorf("Content = %q, want %q", decoded.Content, content)
	}
}

func TestEncodeDecodeParams(t *testing.T) {
	params := map[string]string{
		"REQUEST_METHOD":  "GET",
		"SCRIPT_FILENAME": "/var/www/html/index.php",
		"QUERY_STRING":    "foo=bar&baz=qux",
	}

	encoded := EncodeParams(params)

	decoded, err := DecodeParams(encoded)
	if err != nil {
		t.Fatalf("DecodeParams failed: %v", err)
	}

	if len(decoded) != len(params) {
		t.Errorf("len(decoded) = %d, want %d", len(decoded), len(params))
	}

	for key, expectedValue := range params {
		actualValue, ok := decoded[key]
		if !ok {
			t.Errorf("Missing key %q", key)
		}
		if actualValue != expectedValue {
			t.Errorf("Value for %q = %q, want %q", key, actualValue, expectedValue)
		}
	}
}

// TestParamEncoding tests if params are encoded/decoded correctly
func TestParamEncoding(t *testing.T) {
	params := map[string]string{
		"SCRIPT_FILENAME": "/home/maurits/projects/tqserver/workers/blog/public/hello.php",
		"REQUEST_METHOD":  "GET",
		"REDIRECT_STATUS": "200",
	}

	// Encode params
	encoded := EncodeParams(params)
	log.Printf("Encoded params length: %d bytes", len(encoded))

	// Decode params
	decoded, err := DecodeParams(encoded)
	if err != nil {
		t.Fatalf("Failed to decode params: %v", err)
	}

	// Verify all params match
	if len(decoded) != len(params) {
		t.Errorf("Decoded params count = %d, want %d", len(decoded), len(params))
	}

	for k, v := range params {
		if decoded[k] != v {
			t.Errorf("Param %s = %q, want %q", k, decoded[k], v)
		}
	}
}
