#!/bin/sh
script_dir="$(dirname "$0")"
. "${script_dir}/../env"
cargo run --manifest-path="${script_dir}/Cargo.toml" --release -- --latitude=${LATITUDE} --longitude=${LONGITUDE} --data="${script_dir}/../data/current.csv"
