#!/bin/bash
set -euo pipefail
script_dir="$(realpath "$(dirname "$0")")"
data_dir="${script_dir}/../data"
. "${script_dir}/../env"

rm -f "${data_dir}/current.csv.new"
for location in "${LOCATIONS[@]}"; do
    IFS=, read -ra latlng <<< "${location}"
    go run "${script_dir}/main.go" \
        -latitude="${latlng[0]}" \
        -longitude="${latlng[1]}" \
        >> "${data_dir}/current.csv.new"
done
mv -f "${data_dir}/current.csv.new" "${data_dir}/current.csv"

[ -f "${data_dir}/history.csv" ] && cp -f "${data_dir}/history.csv" "${data_dir}/history.csv.new"
cat "${data_dir}/current.csv" >> "${data_dir}/history.csv.new"
mv -f "${data_dir}/history.csv.new" "${data_dir}/history.csv"
