#!/bin/sh
# Builds the upstream ioq3 Emscripten client (WASM) with the emsdk container.
# Usage: engine/build.sh [ioq3-checkout-dir]  (default: ./.ioq3-src, cloned if missing)
set -eu
SRC=${1:-.ioq3-src}
[ -d "$SRC" ] || git clone --depth 1 https://github.com/ioquake/ioq3 "$SRC"
docker run --rm -v "$(cd "$SRC" && pwd)":/src -w /src emscripten/emsdk:latest sh -c '
  emcmake cmake -S . -B build-em -DCMAKE_BUILD_TYPE=Release -DBUILD_STANDALONE=ON &&
  cmake --build build-em --parallel 4'
echo "output: $SRC/build-em/Release/{ioquake3.js,ioquake3.wasm,baseq3/vm}"
