#!/bin/sh

# write an install script read from stdin
# arg 1: script name
installscript() {
	local _script=${SCRIPTS}/$1
	cat >${_script}
	chmod 0755 ${_script}
}

[ $(uname) = Darwin ] || {
	echo "$0 must be run from darwin" 2>&1
	exit 1
}
[ $# = 4 ] || {
	echo "usage: $0 version identity keychain-profile arch" 2>&1
	exit 2
}

VERSION=$1
IDENTITY=$2
ARCH=$4
KEYCHAINPROFILE=$3
DIST=dist/darwin
SCRIPTS=darwin/scripts
EXE=dcrinstall-darwin-${ARCH}-${VERSION}
BUILD=dist/dcrinstall-${VERSION}/${EXE}
PREFIX=${PREFIX:-/usr/local}

[ -x ${BUILD} ] || go run . -dist dcrinstall -target darwin/${ARCH}
[ -x ${BUILD} ] || {
	echo "cannot package ${BUILD}: executable missing" 2>&1
	exit 1
}

set -ex
[ -d ${DIST} ] && rm -rf ${DIST}
[ -d ${SCRIPTS} ] && rm -rf ${SCRIPTS}
mkdir -p ${DIST}
mkdir -p ${SCRIPTS}

# prepare directory with package files
install -m 0755 ${BUILD} ${DIST}/dcrinstall
[ $ARCH = arm64 ] && codesign --remove-signature ${DIST}/dcrinstall
codesign -s ${IDENTITY} --options runtime ${DIST}/dcrinstall
installscript postinstall <<EOF
#!/bin/sh
echo ${PREFIX}/decred > /etc/paths.d/decred
EOF

# generate signed package for notarization
pkgbuild --identifier org.decred.dcrinstall \
	--version ${VERSION} \
	--root ${DIST} \
	--install-location ${PREFIX}/decred \
	--scripts ${SCRIPTS} \
	--sign ${IDENTITY} \
	dist/dcrinstall-${VERSION}/${EXE}.pkg

# submit notarization
xcrun notarytool submit dist/dcrinstall-${VERSION}/${EXE}.pkg \
		--wait \
		--keychain-profile ${KEYCHAINPROFILE} 2>&1 
	
# staple package with notarization ticket
stapler staple dist/dcrinstall-${VERSION}/${EXE}.pkg
