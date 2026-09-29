#!/bin/sh
# Copyright The Orca Authors
# SPDX-License-Identifier: Apache-2.0

set -eu

# Create the bucket the Registry and Harness keep files in, unless it exists,
# so every `ork local start` can run this. curl signs the S3 requests with
# SigV4 itself, so no S3 client is needed.
bucket="${S3_ENDPOINT}/${S3_BUCKET}"
if curl -fs -o /dev/null --head \
  --aws-sigv4 "aws:amz:${S3_REGION}:s3" \
  --user "${S3_ACCESS_KEY_ID}:${S3_SECRET_ACCESS_KEY}" "${bucket}"; then
  echo "Bucket ${S3_BUCKET} exists."
else
  curl -fsS -o /dev/null -X PUT \
    --aws-sigv4 "aws:amz:${S3_REGION}:s3" \
    --user "${S3_ACCESS_KEY_ID}:${S3_SECRET_ACCESS_KEY}" "${bucket}"
  echo "Created bucket ${S3_BUCKET}."
fi
