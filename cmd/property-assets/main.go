package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"time"

	propertyupload "github.com/infrai-examples/property-asset-upload/internal/property_upload"
)

const bucketName = "property-assets"

func main() {
	apiKey := os.Getenv("INFRAI_API_KEY")
	if apiKey == "" {
		log.Fatal("INFRAI_API_KEY is required")
	}
	client := propertyupload.NewClient(apiKey)
	uploads := propertyupload.NewUploadService(bucketName, client)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /upload-intents", func(w http.ResponseWriter, r *http.Request) {
		var input propertyupload.UploadIntent
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&input); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON request"})
			return
		}
		grant, err := uploads.CreateIntent(r.Context(), input)
		if err != nil {
			status := http.StatusBadRequest
			if !propertyupload.IsClientError(err) && !isDomainError(err) {
				status = http.StatusBadGateway
			}
			writeJSON(w, status, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, grant)
	})

	server := &http.Server{Addr: ":8080", Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	log.Printf("property asset signer listening on %s", server.Addr)
	log.Fatal(server.ListenAndServe())
}

func isDomainError(err error) bool {
	var target *propertyupload.InfraiError
	return !errors.As(err, &target)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
