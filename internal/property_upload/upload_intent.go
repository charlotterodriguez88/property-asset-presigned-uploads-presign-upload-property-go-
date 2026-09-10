package propertyupload

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

type AssetKind string

const (
	MaintenancePhoto   AssetKind = "maintenance_photo"
	TenantDocument     AssetKind = "tenant_document"
	InspectionReminder AssetKind = "inspection_reminder"
)

type UploadIntent struct {
	PropertyID  string    `json:"property_id"`
	RecordID    string    `json:"record_id"`
	Kind        AssetKind `json:"kind"`
	Filename    string    `json:"filename"`
	ContentType string    `json:"content_type"`
}

type UploadGrant struct {
	Method    string `json:"method"`
	UploadURL string `json:"upload_url"`
	ObjectKey string `json:"object_key"`
	MaxBytes  int64  `json:"max_bytes"`
}

type presigner interface {
	PresignPut(context.Context, string, string, PresignPutInput) (PresignResult, error)
}

type UploadService struct {
	bucket    string
	presigner presigner
}

func NewUploadService(bucket string, signer presigner) *UploadService {
	return &UploadService{bucket: bucket, presigner: signer}
}

type uploadPolicy struct {
	prefix   string
	maxBytes int64
	types    map[string]bool
}

var safeID = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

func (s *UploadService) CreateIntent(ctx context.Context, intent UploadIntent) (UploadGrant, error) {
	policy, err := policyFor(intent.Kind)
	if err != nil {
		return UploadGrant{}, err
	}
	if !safeID.MatchString(intent.PropertyID) || !safeID.MatchString(intent.RecordID) {
		return UploadGrant{}, errors.New("property_id and record_id must contain letters, digits, underscore, or dash")
	}
	if !policy.types[intent.ContentType] {
		return UploadGrant{}, fmt.Errorf("content_type %q is not accepted for %s", intent.ContentType, intent.Kind)
	}
	name := filepath.Base(strings.TrimSpace(intent.Filename))
	if name == "." || name == "" {
		return UploadGrant{}, errors.New("filename is required")
	}
	key := fmt.Sprintf("properties/%s/%s/%s/%s", intent.PropertyID, policy.prefix, intent.RecordID, name)
	digest := sha256.Sum256([]byte(string(intent.Kind) + "\x00" + key + "\x00" + intent.ContentType))
	idempotencyKey := "asset-" + hex.EncodeToString(digest[:16])
	result, err := s.presigner.PresignPut(ctx, s.bucket, key, PresignPutInput{
		ContentType: intent.ContentType, MaxBytes: policy.maxBytes, IdempotencyKey: idempotencyKey,
	})
	if err != nil {
		return UploadGrant{}, err
	}
	return UploadGrant{Method: "PUT", UploadURL: result.URL, ObjectKey: key, MaxBytes: policy.maxBytes}, nil
}

func policyFor(kind AssetKind) (uploadPolicy, error) {
	switch kind {
	case MaintenancePhoto:
		return uploadPolicy{"maintenance", 12 << 20, map[string]bool{"image/jpeg": true, "image/png": true}}, nil
	case TenantDocument:
		return uploadPolicy{"tenant-documents", 20 << 20, map[string]bool{"application/pdf": true}}, nil
	case InspectionReminder:
		return uploadPolicy{"inspection-reminders", 8 << 20, map[string]bool{"image/jpeg": true, "application/pdf": true}}, nil
	default:
		return uploadPolicy{}, fmt.Errorf("unknown asset kind %q", kind)
	}
}
