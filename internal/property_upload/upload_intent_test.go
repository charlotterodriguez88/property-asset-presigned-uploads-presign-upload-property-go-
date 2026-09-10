package propertyupload

import (
	"context"
	"testing"
)

type recordingPresigner struct {
	key   string
	input PresignPutInput
}

func (r *recordingPresigner) PresignPut(_ context.Context, _, key string, input PresignPutInput) (PresignResult, error) {
	r.key, r.input = key, input
	return PresignResult{URL: "https://upload.example/signed"}, nil
}

func TestCreateIntentAppliesAssetPolicy(t *testing.T) {
	tests := []struct {
		name    string
		intent  UploadIntent
		wantKey string
		wantMax int64
	}{
		{"maintenance image", UploadIntent{"p-42", "req-7", MaintenancePhoto, "leak.jpg", "image/jpeg"}, "properties/p-42/maintenance/req-7/leak.jpg", 12 << 20},
		{"tenant PDF", UploadIntent{"p-42", "lease-3", TenantDocument, "lease.pdf", "application/pdf"}, "properties/p-42/tenant-documents/lease-3/lease.pdf", 20 << 20},
		{"inspection attachment", UploadIntent{"p-42", "rem-9", InspectionReminder, "notice.pdf", "application/pdf"}, "properties/p-42/inspection-reminders/rem-9/notice.pdf", 8 << 20},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			signer := &recordingPresigner{}
			grant, err := NewUploadService("property-assets", signer).CreateIntent(context.Background(), tt.intent)
			if err != nil {
				t.Fatal(err)
			}
			if signer.key != tt.wantKey {
				t.Fatalf("key = %q, want %q", signer.key, tt.wantKey)
			}
			if grant.MaxBytes != tt.wantMax || signer.input.MaxBytes != tt.wantMax {
				t.Fatalf("max bytes = %d, want %d", grant.MaxBytes, tt.wantMax)
			}
			if signer.input.ContentType != tt.intent.ContentType {
				t.Fatalf("content type = %q", signer.input.ContentType)
			}
			if signer.input.IdempotencyKey == "" {
				t.Fatal("idempotency key is empty")
			}
		})
	}
}

func TestCreateIntentRejectsDocumentImage(t *testing.T) {
	_, err := NewUploadService("property-assets", &recordingPresigner{}).CreateIntent(context.Background(), UploadIntent{
		PropertyID: "p-42", RecordID: "lease-3", Kind: TenantDocument, Filename: "scan.png", ContentType: "image/png",
	})
	if err == nil {
		t.Fatal("expected tenant document policy rejection")
	}
}
