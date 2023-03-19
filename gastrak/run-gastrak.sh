#!/bin/bash
set -euo pipefail
script_dir="$(realpath "$(dirname "$0")")"
data_dir="${script_dir}/../data"
tmp_path="$(mktemp)"
out_path="${data_dir}/history/$(date +%Y)/$(date +%Y%m%d_%H%M%S.csv)"
curr_path="${data_dir}/current.csv"
. "${script_dir}/../env"

for location in "${LOCATIONS[@]}"; do
    IFS=, read -ra latlng <<< "${location}"
    go run "${script_dir}/main.go" \
        -latitude="${latlng[0]}" \
        -longitude="${latlng[1]}" \
        >> "${tmp_path}"
done

mkdir -p "$(dirname "${out_path}")"
mv -f "${tmp_path}" "${out_path}"
ln -sf "${out_path}" "${curr_path}"
