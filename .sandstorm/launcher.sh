#!/bin/sh

random_string() {
  tr -dc 'a-zA-Z0-9' < /dev/urandom | fold -w 64 | head -n 1
}

API_SIGNING_KEY="$(random_string)"
COOKIE_SECRET="$(random_string)"
MAGICLINK_SECRET="$(random_string)"
export API_SIGNING_KEY COOKIE_SECRET MAGICLINK_SECRET

exec /opt/app/app
