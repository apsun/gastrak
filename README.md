# gastrak

Simple tool that scrapes Costco Gasoline prices in an area and plots
the data using OpenStreetMap.

## Setup

1. Set your location (latitude and longitude) in `env`.
2. Put `data/run-gastrak.sh` into cron (or your scheduling tool of choice).
3. Manually run `data/run-gastrak.sh` once to initialize the data.
4. Run `server/run-server.sh`. Change the port with `ROCKET_PORT=xyz`.

Note that the scraper is written in Go and the server in Rust, so you need
both of those installed. Why? IDK, you go and think of something better.

## License

[WTFPL](http://www.wtfpl.net/)
