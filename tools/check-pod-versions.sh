#!/bin/bash

set -euo pipefail

pods="twtxt.net txt.sour.is twt.nfld.uk tt.vltra.plus f.adi.onl yarn.andrewjvpowell.com we.loveprivacy.club arrakis.netbros.com tw.lohn.in"

printf "Pod Version\n"

for pod in $pods; do
  printf "%s " "$pod"
  if ! curl -fqso - -m 5 -H 'Accept: application/json' "https://${pod}/version" |
    jq -er '.FullVersion'; then
    printf "???\n"
  fi
done
