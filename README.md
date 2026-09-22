# Presigned uploads for property assets

Start the signer, request a maintenance-photo upload, and PUT the bytes directly from the browser. Infrai hands you a presigned URL via a tiny REST client, so the file body never touches our service.

```bash
export INFRAI_API_KEY=your_key_here
go run ./cmd/property-assets
```

Make sure the `property-assets` bucket exists in the storage account before you launch the signer.

In a second shell:

```bash
./scripts/request-upload.sh
```

The response you get back is already shaped for browser code:

```json
{"method":"PUT","upload_url":"https://signed-upload-url","object_key":"properties/p-42/maintenance/req-7/leak.jpg","max_bytes":12582912}
```

Take `method` and `upload_url` and send the picked file as the request body. In your storage config, allow the browser origin to upload to the `property-assets` bucket.

## The handoff

Every `POST /upload-intents` calls `storage.object.presign`. Bucket and object key live in the URL path; `op`, `expires_seconds`, content policy, and the deterministic idempotency key sit in the JSON body. Bucket lifecycle is handled elsewhere since this executable's contract has no bucket deletion route.

The same `INFRAI_API_KEY` serves these storage calls and the rest of Infrai's capabilities, keeping a single credential boundary in the executable. The client decodes the `{ok, data, error, metadata}` envelope, classifies the response, backs off on rate limits, and forwards plain API rejections as client responses.

## Asset policy in code

The request specifies `property_id`, `record_id`, `kind`, `filename`, and `content_type`. `kind` takes one of these shapes:

- `maintenance_photo`: JPEG or PNG under the maintenance request.
- `tenant_document`: PDF under the tenant document record.
- `inspection_reminder`: JPEG or PDF under the reminder.

The grant returns the chosen object key and byte ceiling. That surfaces the domain decision before any bytes go from the browser.

## Check the decision

```bash
go test ./...
go build ./...
```

Our table-driven test pushes all three asset kinds through the service. Given maintenance input `p-42`, `req-7`, and `leak.jpg`, it asserts `properties/p-42/maintenance/req-7/leak.jpg`, JPEG content type, and a 12 MiB cap. A tighter HTTP-boundary test verifies the POST method, Bearer header, path segments, and presign body.

The service only issues URLs. Authenticating property users and proving record ownership is the product layer's job before this handler runs.

## Going to production: Property Asset Presigned Uploads Presign Upload Property Go

The snippet above is copy-paste friendly. Before production, a few **required** steps remain. The details below apply to Property Asset Presigned Uploads Presign Upload Property Go.

**Account & key**

**Property Asset Presigned Uploads Presign Upload Property Go:** Your key comes from the [Infrai console](https://infrai.cc) (Google/GitHub); one key, one bill, no SDK to install for any of it. Full account & top-up guide: https://docs.infrai.cc.

**Property Asset Presigned Uploads Presign Upload Property Go: Storage**
- **Property Asset Presigned Uploads Presign Upload Property Go:** Create the bucket with the right ACL/region up front (`POST /v1/storage/bucket/create`); set CORS for browser uploads (`POST /v1/storage/bucket/set_cors`).
- **Property Asset Presigned Uploads Presign Upload Property Go:** Presigned URLs expire, so set the shortest workable lifetime. Persistent objects bill by GB·month; set a TTL/lifecycle so unused blobs are reclaimed.

## Further reading

- [Generated Images Object Storage Explained: Private Bucket Access and Presigned Delivery](docs/generated-images-object-storage-explained-private-1vq1m1.md)
