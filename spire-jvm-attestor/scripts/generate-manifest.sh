#!/usr/bin/env bash
# generate-manifest.sh
#
# Generates the jvm-hashes.json manifest consumed by the SPIRE JVM Attestor.
# Run this script in your CI/CD pipeline after docker build, before image push.
#
# Usage:
#   ./generate-manifest.sh <jar-path-inside-container> <output-file>
#
# Example:
#   ./generate-manifest.sh /app/payments-service.jar ./jvm-hashes.json
#
# The output file should be mounted into the SPIRE Agent container at the path
# configured in agent.conf: hash_manifest_path = "/etc/spire/jvm-hashes.json"

set -euo pipefail

JAR_PATH="${1:-/app/payments-service.jar}"
OUTPUT="${2:-jvm-hashes.json}"

if [[ ! -f "$JAR_PATH" ]]; then
  echo "ERROR: jar not found: $JAR_PATH" >&2
  exit 1
fi

HASH=$(sha256sum "$JAR_PATH" | awk '{print $1}')
TIMESTAMP=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
GIT_SHA=$(git rev-parse HEAD 2>/dev/null || echo "unknown")

cat > "$OUTPUT" <<EOF
{
  "version": 1,
  "generated_at": "${TIMESTAMP}",
  "generated_by": "ci/build@sha:${GIT_SHA}",
  "jars": {
    "$(basename "$JAR_PATH")": "${HASH}"
  }
}
EOF

echo "Manifest written to ${OUTPUT}"
echo "  jar:  ${JAR_PATH}"
echo "  hash: ${HASH:0:16}..."
