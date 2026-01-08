package fastcgi

import (
	"log"
	"testing"
)

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
