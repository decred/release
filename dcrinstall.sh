#!/bin/sh

set -e

TUPLE=$(go version | perl -lane 'print $F[-1] =~ s,/,-,r')
DCRINSTALL=./bin/${TUPLE}/dcrinstall

[ -x ${DCRINSTALL} ] || go run . -dist dcrinstall
[ -f fake-latest ] || go run . -dist dcrinstall-manifests

exec ${DCRINSTALL} -manifest file://fake-latest -skippgp "$@"
