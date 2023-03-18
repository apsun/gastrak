#!/bin/bash
set -euo pipefail
script_dir="$(realpath "$(dirname "$0")")"
data_dir="${script_dir}/../data"
. "${script_dir}/../env"

cd "${script_dir}"
IFS=, read -ra latlng <<< "${LOCATIONS[0]}"
go run "${script_dir}/main.go" \
    -latitude="${latlng[0]}" \
    -longitude="${latlng[1]}" \
    -data="${data_dir}/current.csv" \
    -history="${data_dir}/history.csv" \
    -port="${SERVER_PORT}"
