#!/bin/sh
set -eu

curl --fail-with-body --request POST http://localhost:8080/upload-intents \
  --header 'Content-Type: application/json' \
  --data '{"property_id":"p-42","record_id":"req-7","kind":"maintenance_photo","filename":"leak.jpg","content_type":"image/jpeg"}'
