#!/bin/sh

if [ $# -ne 4 ]; then
  echo "Usage: $(basename "$0") <source_pod> <target_pod> <twt_hash>"
  exit 1
fi

source_pod="$1"
target_pod="$2"
twt_hash="$3"

curl -qso - -H 'Accept: application/json' "$source_pod/twt/$twt_hash" \
  | curl -vo - -X POST \
  -H "Token:$YARND_TOKEN" -H "Accept: application/json" \
  -H "Content-type:application/json" \
  --data-binary @- "$target_pod/api/v1/inject
