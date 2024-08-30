#!/bin/sh

set -e

TUPLE=$(go version | perl -lane 'print $F[-1] =~ s,/,-,r')

for APP in dcrd dcrctl dcrwallet bisonw bisonw-tray bwctl dcrlncli dcrlnd politeiavoter; do
	APP="./bin/${TUPLE}/${APP}"
	[ -x "${APP}" ] && "${APP}" --version || echo "WARNING: ${APP} is not built"
done
