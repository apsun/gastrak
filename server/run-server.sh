#!/bin/sh
script_dir="$(dirname "$0")"
. "${script_dir}/../env"
cd "${script_dir}"
go run main.go \
    --latitude=${LATITUDE} \
    --longitude=${LONGITUDE} \
    --data="${script_dir}/../data/current.csv" \
    --history="${script_dir}/../data/history.csv" \
    --port="${SERVER_PORT}"
