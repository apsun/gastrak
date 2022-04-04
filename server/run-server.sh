#!/bin/bash
set -euo pipefail
script_dir="$(dirname "$0")"
. "${script_dir}/../env"

cd "${script_dir}"
IFS=, read -ra latlng <<< "${LOCATIONS[0]}"
go run "main.go" \
    --latitude="${latlng[0]}" \
    --longitude="${latlng[1]}" \
    --data="../data/current.csv" \
    --history="../data/history.csv" \
    --port="${SERVER_PORT}"
