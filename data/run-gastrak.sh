#!/bin/sh
script_dir="$(dirname "$0")"
. "${script_dir}/../env"
go run "${script_dir}/gastrak.go" -latitude=${LATITUDE} -longitude=${LONGITUDE} | tee "${script_dir}/current.csv" >> "${script_dir}/history.csv"
