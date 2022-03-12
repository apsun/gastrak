#!/bin/sh
set -euo pipefail
script_dir="$(dirname "$0")"
. "${script_dir}/../env"

go run "${script_dir}/gastrak.go" -latitude=${LATITUDE} -longitude=${LONGITUDE} > "${script_dir}/current.csv.new"
cat "${script_dir}/current.csv.new" >> "${script_dir}/history.csv"
mv "${script_dir}/current.csv.new" "${script_dir}/current.csv"
