#!/bin/bash
set -euo pipefail
script_dir="$(dirname "$0")"
. "${script_dir}/../env"

rm -f "${script_dir}/current.csv.new"
for location in "${LOCATIONS[@]}"; do
    IFS=, read -ra latlng <<< "${location}"
    go run "${script_dir}/gastrak.go" \
        -latitude="${latlng[0]}" \
        -longitude="${latlng[1]}" \
        >> "${script_dir}/current.csv.new"
done
mv -f "${script_dir}/current.csv.new" "${script_dir}/current.csv"

[ -f "${script_dir}/history.csv" ] && cp -f "${script_dir}/history.csv" "${script_dir}/history.csv.new"
cat "${script_dir}/current.csv" >> "${script_dir}/history.csv.new"
mv -f "${script_dir}/history.csv.new" "${script_dir}/history.csv"
