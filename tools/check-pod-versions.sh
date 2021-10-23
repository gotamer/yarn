#!/bin/bash

set -euo pipefail

pods="twtxt.net txt.sour.is twt.nfld.uk tt.vltra.plus f.adi.onl yarn.andrewjvpowell.com we.loveprivacy.club"

printf "Pod Version\n"

for pod in $pods; do
  printf "%s " "$pod"
  if ! curl -fqso - -H 'Accept: application/json' "https://${pod}/version" |
    jq -er '.FullVersion'; then
    printf "???\n"
  fi
done
