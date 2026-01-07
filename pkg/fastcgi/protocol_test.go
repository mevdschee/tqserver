package fastcgi

import (
	"bytes"
	"testing"
)

func TestHeaderEncodeDecode(t *testing.T) {
	tests := []struct {
		name          string
		recType       uint8
		reqID         uint16
		contentLength uint16
	}{
		{"BeginRequest", BeginRequest, 1, 8},
		{"Params", Params, 1, 100},
		{"Stdin", Stdin, 1, 0},
		{"Stdout", Stdout, 1, 256},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			header := NewHeader(tt.recType, tt.reqID, tt.contentLength)
			var buf bytes.Buffer

			if err := header.Encode(&buf); err != nil {
				t.Fatalf("Encode failed: %v", err)
			}

			decoded, err := DecodeHeader(&buf)
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
		Role:  Responder,
		Flags: FlagKeepConn,
	}
	var buf bytes.Buffer

	if err := body.Encode(&buf); err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	decoded, err := DecodeBeginRequestBody(&buf)
	if err != nil {
		t.Fatalf("DecodeBeginRequestBody failed: %v", err)
	}

	if decoded.Role != Responder {
		t.Errorf("Role = %d, want %d", decoded.Role, Responder)
	}
	if decoded.Flags != FlagKeepConn {
		t.Errorf("Flags = %d, want %d", decoded.Flags, FlagKeepConn)
	}
	if !decoded.KeepConn() {
		t.Error("KeepConn() = false, want true")
	}
}

func TestEndRequestBodyEncodeDecode(t *testing.T) {
	body := &EndRequestBody{
		AppStatus:      0,
		ProtocolStatus: RequestComplete,
	}
	var buf bytes.Buffer

	if err := body.Encode(&buf); err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	decoded, err := DecodeEndRequestBody(&buf)
	if err != nil {
		t.Fatalf("DecodeEndRequestBody failed: %v", err)
	}

	if decoded.AppStatus != 0 {
		t.Errorf("AppStatus = %d, want 0", decoded.AppStatus)
	}
	if decoded.ProtocolStatus != RequestComplete {
		t.Errorf("ProtocolStatus = %d, want %d", decoded.ProtocolStatus, RequestComplete)
	}
}

func TestRecordEncodeDecode(t *testing.T) {
	content := []byte("Hello, FastCGI!")
	record := NewRecord(Stdout, 1, content)
	var buf bytes.Buffer

	if err := record.Encode(&buf); err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	decoded, err := DecodeRecord(&buf)
	if err != nil {
		t.Fatalf("DecodeRecord failed: %v", err)
	}

	if decoded.Header.Type != Stdout {
		t.Errorf("Type = %d, want %d", decoded.Header.Type, Stdout)
	}
	if decoded.Header.RequestID != 1 {
		t.Errorf("RequestID = %d, want 1", decoded.Header.RequestID)
	}
	if !bytes.Equal(decoded.Content, content) {
		t.Errorf("Content = %q, want %q", decoded.Content, content)
	}
}

func TestEncodeDecodeParams(t *testing.T) {
	params := map[string]string{
		"SCRIPT_FILENAME": "/var/www/html/index.php",
		"REQUEST_METHOD":  "GET",
		"QUERY_STRING":    "foo=bar",
		"REQUEST_URI":     "/index.php?foo=bar",
	}
	encoded := EncodeParams(params)
	decoded, err := DecodeParams(encoded)
	if err != nil {
		t.Fatalf("DecodeParams failed: %v", err)
	}

	if len(decoded) != len(params) {
		t.Fatalf("len(decoded) = %d, want %d", len(decoded), len(params))
	}
	for name, value := range params {
		if decoded[name] != value {
			t.Errorf("decoded[%q] = %q, want %q", name, decoded[name], value)
		}
	}
}

func TestEncodeParamLongValue(t *testing.T) {
	// Test with value > 127 bytes (requires 4-byte length encoding)
	longValue := string(make([]byte, 200))
	for i := range longValue {
		longValue = "a" + longValue[1:]
	}

	params := map[string]string{
		"LONG_PARAM": longValue,
	}
	encoded := EncodeParams(params)
	decoded, err := DecodeParams(encoded)
	if err != nil {
		t.Fatalf("DecodeParams failed: %v", err)
	}

	if decoded["LONG_PARAM"] != longValue {
		t.Errorf("Long value mismatch")
	}
}
