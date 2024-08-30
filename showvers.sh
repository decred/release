#!/bin/sh

set -e

TUPLE=$(go version | perl -lane 'print $F[-1] =~ s,/,-,r')
OS=$(echo $TUPLE | cut -f1 -d-)

for APP in dcrd dcrctl dcrwallet bisonw bisonw-tray bwctl dcrlncli dcrlnd politeiavoter; do
	if [ "$OS" != "windows" ] && [ "$APP" == "bisonw-tray" ]; then
		continue
	fi
	APP="./bin/${TUPLE}/${APP}"
	[ -x "${APP}" ] && "${APP}" --version || echo "WARNING: ${APP} is not built"
done
