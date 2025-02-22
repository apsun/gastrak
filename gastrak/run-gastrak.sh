#!/bin/bash
set -euo pipefail
script_dir="$(realpath "$(dirname "$0")")"
data_dir="${script_dir}/../data"
tmp_path="$(mktemp)"
out_path="${data_dir}/history/$(date +%Y)/$(date +%Y%m%d_%H%M%S.csv)"
curr_path="${data_dir}/current.csv"
hist_path="${data_dir}/history.db"
. "${script_dir}/../config"

for location in "${LOCATIONS[@]}"; do
    IFS=, read -ra latlng <<< "${location}"
    echo >&2 "Fetching data for (${latlng[0]}, ${latlng[1]})"
    go run "${script_dir}/main.go" \
        -latitude="${latlng[0]}" \
        -longitude="${latlng[1]}" \
        >> "${tmp_path}"
done

mkdir -p "$(dirname "${out_path}")"
mv -f "${tmp_path}" "${out_path}"
ln -sfr "${out_path}" "${curr_path}"

if [ ! -f "${hist_path}" ]; then
    echo >&2 "Initializing history.db from csv; this may take a while"

    sqlite3 "${hist_path}" <<EOF
CREATE TABLE data(
    time INTEGER NOT NULL,
    id INTEGER NOT NULL,
    name TEXT NOT NULL,
    latitude REAL NOT NULL,
    longitude REAL NOT NULL,
    regular_price TEXT NOT NULL,
    premium_price TEXT NOT NULL,
    diesel_price TEXT NOT NULL
);
CREATE INDEX ix_name ON data(name);
EOF

    find "${data_dir}/history" -type f -exec cat {} \; \
        | sqlite3 "${hist_path}" ".import --csv '|cat -' data"
else
    sqlite3 "${hist_path}" ".import --csv '|cat -' data" < "${curr_path}"
fi

if [ "${ENABLE_GIT}" -eq 1 ] && [ -d "${data_dir}/.git" ]; then
    cd "${data_dir}"
    git pull
    git add "${out_path}"
    git commit -m "Updated at $(date "+%Y-%m-%d %H:%M:%S")"
    git push
fi
