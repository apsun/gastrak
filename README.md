# gastrak

Simple tool that scrapes Costco Gasoline prices in an area and plots
the data using OpenStreetMap.

## Setup

1. Set your location (latitude and longitude) in `config`.
2. Put `gastrak/run-gastrak.sh` into cron (or your scheduling tool of choice).
3. Manually run `gastrak/run-gastrak.sh` once to initialize the data.
4. Run `server/run-server.sh`.

## License

[WTFPL](http://www.wtfpl.net/)
