#!/usr/bin/env sh
set -eu
cd "$(dirname "$0")/.."
project="sap-seg-e2e-$(date +%s)-$$"
trap 'docker compose -p "$project" down --volumes --remove-orphans' EXIT HUP INT TERM
docker compose -p "$project" up --build -d postgres mock-erp
docker compose -p "$project" run --build --rm e2e-verify
docker compose -p "$project" --profile test run --rm integration
