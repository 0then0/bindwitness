#!/bin/sh
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
out=${1:-"$root/build/fixtures"}
mkdir -p "$out/a" "$out/b"
cc=${CC:-cc}
src="$root/testdata/native"
"$cc" -Wall -Wextra -Werror -fPIC -shared "$src/provider.c" -Wl,-soname,libsame.so -Wl,--build-id -o "$out/a/libsame.so"
"$cc" -Wall -Wextra -Werror -fPIC -shared "$src/provider.c" -Wl,-soname,libsame.so -Wl,--build-id -o "$out/b/libsame.so"
"$cc" -Wall -Wextra -Werror -fPIC -shared "$src/provider.c" -Wl,--version-script="$src/version.map" -Wl,-soname,libversion.so -Wl,--build-id -o "$out/libversion.so"
"$cc" -Wall -Wextra -Werror -fPIC -shared "$src/provider.c" -DSYMBOL=bw_private_value -Wl,-soname,libprivate.so -Wl,--build-id -o "$out/libprivate.so"
"$cc" -Wall -Wextra -Werror -fPIC -shared "$src/consumer.c" -o "$out/consumer.so" -Wl,--build-id
"$cc" -Wall -Wextra -Werror -fPIC -shared "$src/consumer.c" -L"$out" -lversion '-Wl,-rpath,$ORIGIN' -Wl,--build-id -o "$out/version-consumer.so"
"$cc" -Wall -Wextra -Werror -fPIC -shared "$src/consumer.c" -DSYMBOL=bw_private_value -L"$out" -lprivate '-Wl,-rpath,$ORIGIN' -Wl,--build-id -o "$out/private-consumer.so"
"$cc" -Wall -Wextra -Werror "$src/normal.c" -L"$out/a" -lsame '-Wl,-rpath,$ORIGIN/a' -Wl,--build-id -o "$out/normal"
"$cc" -Wall -Wextra -Werror "$src/host.c" -ldl -Wl,--build-id -o "$out/host"
"$cc" -Wall -Wextra -Werror -fPIC -shared "$src/provider.c" -o "$out/a/librelative.so"
"$cc" -Wall -Wextra -Werror -fPIC -shared "$src/provider.c" -DOFFSET=8 -o "$out/b/librelative.so"
"$cc" -Wall -Wextra -Werror "$src/relative-host.c" -ldl -o "$out/relative-host"
"$cc" -Wall -Wextra -Werror -fPIC -shared "$src/weak-provider.c" -o "$out/libweak.so"
"$cc" -Wall -Wextra -Werror "$src/multi-host.c" -ldl -o "$out/multi-host"
