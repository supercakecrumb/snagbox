#!/usr/bin/env bash
# Single verification gate for snagbox commits.
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/.."

START_TIME=$(date +%s)

# Self-install: make this script run automatically as the git pre-commit hook.
git config core.hooksPath scripts/githooks

section() {
	echo "=== $1 ==="
}

section "gofmt"
UNFORMATTED=$(gofmt -l . | grep -v '^clients/' || true)
if [ -n "$UNFORMATTED" ]; then
	echo "The following files are not gofmt-ed:"
	echo "$UNFORMATTED"
	exit 1
fi

section "go build"
go build ./...

section "go vet"
go vet ./...

section "go test"
go test ./...

section "integration tests"
if [ -f .env ]; then
	set -a
	# shellcheck disable=SC1091
	source .env
	set +a

	if docker exec snagbox-db-1 true >/dev/null 2>&1; then
		docker exec snagbox-db-1 psql -U "${POSTGRES_USER:-snagbox}" -c "CREATE DATABASE snagbox_test" || true

		SNAGBOX_TEST_DATABASE_URL="postgres://${POSTGRES_USER:-snagbox}:${POSTGRES_PASSWORD}@localhost:5432/snagbox_test?sslmode=disable" \
			SNAGBOX_TEST_S3_ENDPOINT="localhost:9000" \
			SNAGBOX_TEST_S3_ACCESS_KEY="${MINIO_ROOT_USER:-snagbox}" \
			SNAGBOX_TEST_S3_SECRET_KEY="${MINIO_ROOT_PASSWORD}" \
			go test -p 1 -tags integration ./...
	else
		echo "NOTICE: snagbox-db-1 container not reachable, skipping integration tests"
	fi
else
	echo "NOTICE: .env not found, skipping integration tests"
fi

section "golangci-lint"
golangci-lint run ./...

section "changie"
CHANGIE_OUTPUT=$(changie batch --dry-run patch 2>&1) || {
	if echo "$CHANGIE_OUTPUT" | grep -qi "no unreleased changes"; then
		echo "$CHANGIE_OUTPUT"
	else
		echo "$CHANGIE_OUTPUT"
		exit 1
	fi
}

section "staged secrets scan"
STAGED_FILES=$(git diff --cached --name-only)
BLOCKED=$(echo "$STAGED_FILES" | grep -E '(^|/)\.env$|\.log$|\.dump$|(^|/)data/' || true)
if [ -n "$BLOCKED" ]; then
	echo "Refusing to commit files that look like secrets or local data:"
	echo "$BLOCKED"
	exit 1
fi

END_TIME=$(date +%s)
ELAPSED=$((END_TIME - START_TIME))
echo "=== pre-commit checks passed in ${ELAPSED}s ==="
