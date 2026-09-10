package propertyupload

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPresignPutRequestBoundary(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s", r.Method)
		}
		wantPath := "/v1/storage/object/presign/" + "property-assets/properties/p-42/maintenance/req-7/leak.jpg"
		if r.URL.Path != wantPath {
			t.Errorf("path = %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("authorization header missing")
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["op"] != "put" || body["expires_seconds"] != float64(600) {
			t.Errorf("body = %#v", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"data":{"url":"https://upload.example/signed"},"error":null,"metadata":{}}`))
	}))
	defer server.Close()
	c := NewClient("test-key")
	c.baseURL, c.http = server.URL, server.Client()
	got, err := c.PresignPut(context.Background(), "property-assets", "properties/p-42/maintenance/req-7/leak.jpg", PresignPutInput{"image/jpeg", 12 << 20, "asset-fixed"})
	if err != nil {
		t.Fatal(err)
	}
	if got.URL != "https://upload.example/signed" {
		t.Fatalf("url = %q", got.URL)
	}
}
