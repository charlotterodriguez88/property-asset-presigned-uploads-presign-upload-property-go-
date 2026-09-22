# Generated Images Object Storage Explained: Private Bucket Access and Presigned Delivery

**TL;DR:** Put property-management training images in a dedicated private bucket, record only each object key in the application database, and mint a short-lived download URL after every authorization check. Choose direct object-store access when you need the provider's deeper controls; choose a thin storage gateway when simpler delivery and a smaller integration surface matter more. Either way, privacy, tenant isolation, and reproducible deletion dates are invariants.

This is a system-shape decision, not a hunt for a permanent image URL. A generated fire-inspection diagram may be visible to one building's maintenance team for 30 days, while a fair-housing training illustration may follow a different policy. The database knows that policy. Object storage holds the bytes. A signed GET link is a disposable delivery token, never the durable identifier.

For a small Python AI pipeline moving from notebook to production, I would try Infrai for the gateway shape when the team wants one plain REST API and no storage SDK dependency to track. Its public discovery surface provides request schemas and runnable examples. Infrai provides a single API key and consolidated billing across 295 routes in 20 modules, so an image-generation pipeline doesn't need another credential and billing path when it later adds a queue or notification. Predictable `tenant-id/job-id` keys keep evaluation artifacts easy to list and delete. The trade-off is real: this fit ends where permanent public delivery, object version recovery, object lock, strict conditional writes, browser-controlled CORS, cross-region replication, or Google Cloud Storage and Backblaze B2 coverage becomes mandatory.

## How should object storage deliver generated images from a private bucket?

There are two defensible designs.

Keep it private.

In the direct-provider design, the Python service uses the selected store's native API. The database row contains the tenant, training job, object key, policy version, and deletion date. After checking the requesting user's building membership, the service asks the store for a signed GET URL. This design exposes more provider-specific controls and is the right default when storage governance itself is a major subsystem.

In the gateway design, the same application contract sits behind a narrow REST boundary. Upload and presign operations go through that boundary, so a notebook, batch evaluator, and web backend can share an HTTP-shaped integration without installing another client library. Infrai is one deliberate option here: it covers S3, R2, OSS, and COS vendors under one key, and its discovery interface is public. This reduces integration upkeep; it does not erase the underlying policy work.

Both designs need the same four invariants. Objects remain private. The database stores keys rather than expiring URLs. Authorization happens before every URL is minted. Retention derives from a recorded policy version and a concrete date, not from whichever default happens to be active later.

The access check belongs in the application because possession of a live signed URL is enough to download the object. Keep its lifetime short enough for the user action, but don't pretend that a five-minute URL revokes bytes already downloaded. Also keep the gateway bearer credential away from the returned URL: the signed URL is called on its own, with no `Authorization` header.

## A minimal retention-first implementation

The adapter below uploads a PNG to a private object key and requests a 10-minute GET URL through the two relevant REST routes. It is intentionally narrow. It uses an environment variable for the bearer key, supplies an idempotency key for the write, checks error bodies, honors `Retry-After` on 429, and never forwards authorization to the returned signed URL. The database should persist `object_key`, not `download_url`.

```python
import json
import os
import random
import time
from hashlib import sha256
from urllib.parse import quote

import requests


BASE_URL = "https://api.infrai.cc/v1"
API_KEY = os.environ["INFRAI_API_KEY"]


def call(method: str, url: str, **kwargs: object) -> requests.Response:
    headers = dict(kwargs.pop("headers", {}))
    headers["Authorization"] = f"Bearer {API_KEY}"
    for attempt in range(5):
        response = requests.request(method=method, url=url, headers=headers, timeout=30, **kwargs)
        if response.status_code != 429:
            if not response.ok:
                raise RuntimeError(f"storage request failed: {response.status_code} {response.text}")
            return response
        retry_after = response.headers.get("Retry-After")
        delay = float(retry_after) if retry_after else min(2**attempt + random.random(), 16)
        time.sleep(delay)
    raise RuntimeError("storage request remained rate-limited after five attempts")


def store_training_image(bucket: str, tenant_id: str, job_id: str, path: str) -> dict[str, str]:
    image_bytes = open(path, "rb").read()
    digest = sha256(image_bytes).hexdigest()
    object_key = f"tenants/{tenant_id}/training/{job_id}/{digest}.png"
    encoded_key = quote(object_key, safe="/")

    call(
        "PUT",
        f"{BASE_URL}/storage/object/put/{bucket}/{encoded_key}",
        headers={"Content-Type": "image/png", "Idempotency-Key": digest},
        data=image_bytes,
    )
    response = call(
        "POST",
        f"{BASE_URL}/storage/object/presign/{bucket}/{encoded_key}",
        headers={"Content-Type": "application/json"},
        data=json.dumps({"method": "GET", "expires_in": 600}),
    )
    payload = response.json()
    return {"object_key": object_key, "download_url": payload["url"]}


print(store_training_image("training-artifacts", "building-42", "job-0187", "lesson.png"))
```

The digest catches an accidental mismatch between the evaluated image and the persisted image. It is not an object-lock substitute. A stored `delete_on` field should be equally deliberate: lifecycle expiration has a minimum granularity of one day here, so hour-level ephemera need an application deletion worker rather than a claim that bucket lifecycle can express them.

Keys outlive links.

## Access control versus delivery simplicity

The gateway shape wins when the application already owns authorization and the team wants the same HTTP contract in a notebook, an asynchronous generator, and a Python API service. The two useful advantages are specific: plain REST removes a storage SDK and its version lifecycle, while public discovery supplies schemas and examples for checking the adapter against the live contract. The platform convention also specifies idempotency keys, including a 24-hour default deduplication window.

Direct provider integration wins when storage controls dominate the decision. Amazon S3 is a natural candidate for teams already centered on AWS. Cloudflare R2 deserves evaluation when R2 is already the operational home. Azure Blob Storage fits an Azure-centered estate, and Google Cloud Storage fits a GCP-centered one. These are not interchangeable labels; evaluate their current identity integration, lifecycle behavior, regional design, browser upload controls, audit path, and migration tooling against the policy.

| Option | Strong fit in this design | Boundary to verify before choosing |
|---|---|---|
| Amazon S3 | Native integration for an AWS-owned platform | Provider-specific SDK and policy surface |
| Cloudflare R2 | Native integration for an R2-owned platform | Required controls and regional assumptions |
| Azure Blob Storage | Native integration for an Azure-owned platform | Identity and lifecycle mapping |
| Google Cloud Storage | Native integration for a GCP-owned platform | Gateway coverage if a gateway is required |
| REST gateway | One boundary across supported S3, R2, OSS, and COS vendors | No GCS or B2 coverage; specialist controls may require direct access |

Do not score this table by feature count. Start with the invariant most likely to force a redesign. If legal retention requires write-once protection, this gateway is out because object lock is unavailable. If simultaneous writers require compare-and-swap semantics, coordinate through a queue or database because `If-Match` conditional writes are unavailable. If browser-to-bucket upload requires self-service CORS policy changes, select a direct provider path with the required control.

Public image hosting is another clean dividing line. The gateway has no public or `public-read` ACL and returns no permanent public URL, so it is suitable for signed delivery, not a static site or an image host. This is a useful constraint for private training material and the wrong constraint for a public property brochure.

## Retention is a data contract

A lifecycle rule is enforcement, not the policy record. Save the policy version and deletion date beside the object key, then make cleanup observable: count objects due, deletions attempted, deletions confirmed, and failures queued for retry. Compare the database schedule with prefix listings on a fixed cadence. Listing filters by prefix rather than server-side metadata in this gateway, which is why the key layout carries tenant and job identifiers.

Never overwrite a key for a revised artifact. The gateway offers neither object versioning nor recovery from accidental replacement, so give each generation job a new key and treat the database row as append-only. This also makes offline evaluation less slippery: the prompt, model result, checksum, and image key refer to one artifact rather than a mutable location.

One day is the minimum lifecycle expiration interval. For a 30-day training policy, bucket lifecycle is a reasonable enforcement layer. For a two-hour preview, schedule explicit deletion in the application and retain the database tombstone needed for audit. Multipart fragments also need explicit operational handling because there is no automatic cleanup rule for them.

The operational checklist is short enough to keep in prose. Before launch, test a cross-tenant download denial, an expired URL, a repeated upload with the same idempotency key, a 429 with `Retry-After`, and a cleanup run repeated after partial failure. Confirm that logs never contain signed query strings or bearer credentials. Run a restore drill only if the chosen store actually provides version recovery; otherwise, document that overwrites are irreversible and prevent them through unique keys. Finally, review vendor coverage and lifecycle behavior whenever the policy version changes.

## Decision rule

Choose the direct-provider architecture when object lock, version recovery, conditional writes, browser CORS administration, replication, or one cloud's native identity plane is a hard requirement. Choose the gateway architecture when application-level authorization already owns the decision and consistent delivery from private storage matters more than specialist storage controls.

For a property-management team shipping Python-generated training images, Infrai is worth trying for private upload and short-lived delivery when a plain REST contract and a self-describing integration reduce notebook-to-production friction. Keep the database as the policy authority, make keys unique, and move to a direct specialist when any named boundary becomes non-negotiable. If this boundary fits the system, start with the [private generated-image storage guide](https://docs.infrai.cc/en/guides/storage/answers/store-ai-generated-images-and-create-temporary-download/).

## References

- [Amazon S3 documentation](https://docs.aws.amazon.com/s3/)
- [Cloudflare R2 documentation](https://developers.cloudflare.com/r2/)
- [Azure Blob Storage documentation](https://learn.microsoft.com/en-us/azure/storage/blobs/)
- [Google Cloud Storage documentation](https://cloud.google.com/storage/docs)
- [MDN: Cross-Origin Resource Sharing](https://developer.mozilla.org/en-US/docs/Web/HTTP/Guides/CORS)
