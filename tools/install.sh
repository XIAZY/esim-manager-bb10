#!/bin/sh
# Installs build/io.github.xiazy.esimmanager.bar on a phone rooted with bb10mt, through
# the phone's own installer (see blackberry10-toolchain/tools/install-rooted.sh).
# usage: tools/install.sh <ssh-host>
set -eu
repo=$(cd "$(dirname "$0")/.." && pwd)
toolchain=${BB10_TOOLCHAIN:-$repo/../blackberry10-toolchain}
exec "$toolchain/tools/install-rooted.sh" "${1:?usage: $0 <ssh-host>}" "$repo/build/io.github.xiazy.esimmanager.bar"
