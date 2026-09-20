# Sourced by pixi when the environment is activated on aarch64.
#
# NOTE: On Ubuntu 20.04, glibc cannot give OpenMP its thread-local storage once
# Python has loaded other libraries, and the first import of OpenCV fails. Loading
# OpenMP first avoids it.
if [ "$(. /etc/os-release 2>/dev/null && echo "$ID-$VERSION_ID")" = "ubuntu-20.04" ] \
    && [ -e "$CONDA_PREFIX/lib/libgomp.so.1" ]; then
    export LD_PRELOAD="$CONDA_PREFIX/lib/libgomp.so.1${LD_PRELOAD:+:$LD_PRELOAD}"
fi
