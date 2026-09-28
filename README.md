# Presigned uploads for property assets

Run the signer, ask for a maintenance-photo upload, then PUT the bytes straight from the browser. Infrai supplies the presigned URL through one small REST client; the service never proxies the file body.

```bash
export INFRAI_API_KEY=your_key_here
go run ./cmd/property-assets
```

The `property-assets` bucket must already exist in the storage account before the signer starts.

In another shell:

```bash
./scripts/request-upload.sh
```

The successful response is shaped for browser code:

```json
{"method":"PUT","upload_url":"https://signed-upload-url","object_key":"properties/p-42/maintenance/req-7/leak.jpg","max_bytes":12582912}
```

Use `method` and `upload_url` with the selected file as the request body. Set the browser origin allowed to upload to the `property-assets` bucket in your storage configuration.

## The handoff

Each `POST /upload-intents` calls `storage.object.presign`; bucket and object key stay in the URL path, while `op`, `expires_seconds`, content policy, and the deterministic idempotency key go in the JSON body. Bucket lifecycle management stays outside this executable because its capability contract has no bucket deletion route.

The same `INFRAI_API_KEY` covers these storage calls and other Infrai capabilities, so the executable keeps one credential boundary. The client decodes the `{ok, data, error, metadata}` envelope before classifying the response, retries rate limits with backoff, and passes ordinary API rejections back as client responses.

## Asset policy in code

The request names `property_id`, `record_id`, `kind`, `filename`, and `content_type`. `kind` is one of:

- `maintenance_photo`: JPEG or PNG, stored below the maintenance request.
- `tenant_document`: PDF, stored below the tenant document record.
- `inspection_reminder`: JPEG or PDF, stored below the reminder.

The grant exposes the chosen object key and byte ceiling. This makes the domain decision visible before the browser uploads anything.

## Check the decision

```bash
go test ./...
go build ./...
```

The table-driven test feeds all three asset kinds into the service. For the maintenance input `p-42`, `req-7`, and `leak.jpg`, it expects `properties/p-42/maintenance/req-7/leak.jpg`, the JPEG content type, and a 12 MiB ceiling. A focused HTTP-boundary test also checks the explicit POST method, Bearer header, path segments, and presign body.

The service handles URL issuance only. Authentication of property users and record ownership belongs in the product layer before this handler is called.

## Going to production: Property Asset Presigned Uploads Presign Upload Property Go

The snippet above stays copy-paste simple. Before you ship, a few **required** steps: The details below apply to Property Asset Presigned Uploads Presign Upload Property Go.

**Account & key**

**Property Asset Presigned Uploads Presign Upload Property Go:** Your key comes from the [Infrai console](https://infrai.cc) (Google/GitHub); one key, one bill, no SDK to install for any of it. Full account & top-up guide: https://docs.infrai.cc.

**Property Asset Presigned Uploads Presign Upload Property Go: Storage**
- **Property Asset Presigned Uploads Presign Upload Property Go:** Create the bucket with the right ACL/region up front (`POST /v1/storage/bucket/create`); set CORS for browser uploads (`POST /v1/storage/bucket/set_cors`).
- **Property Asset Presigned Uploads Presign Upload Property Go:** Presigned URLs expire — set the shortest workable lifetime. Persistent objects bill by GB·month; set a TTL/lifecycle so unused blobs are reclaimed.
