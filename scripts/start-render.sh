#!/bin/sh
set -eu

/usr/local/bin/demo-bootstrap
exec /usr/local/bin/rate-limiter
