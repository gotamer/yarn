#!/bin/bash

set -euo pipefail

get_pod_version() {
  if ! version="$(bat -print=b GET "$1/info" Accept:application/json | jq -er '.software_version' 2> /dev/null)"; then
    version="$(bat -print=b GET "$1/version" Accept:application/json | jq -er '.FullVersion' 2> /dev/null)"
  fi
  echo -n "$version"
}

pods="twtxt.net txt.sour.is twt.nfld.uk tt.vltra.plus yarn.andrewjvpowell.com we.loveprivacy.club arrakis.netbros.com tw.lohn.in yarn.meff.me txt.quisquiliae.com mentano.org"

echo "Pod Version"

for pod in $pods; do
  echo "$pod $(get_pod_version "$pod")"
done
