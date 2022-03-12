#!/bin/sh
set -euo pipefail
script_dir="$(dirname "$0")"
. "${script_dir}/../env"

cd "${script_dir}"
go run "main.go" \
    --latitude=${LATITUDE} \
    --longitude=${LONGITUDE} \
    --data="../data/current.csv" \
    --history="../data/history.csv" \
    --port="${SERVER_PORT}"
