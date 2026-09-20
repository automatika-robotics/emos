# Sourced by pixi when the environment is activated on aarch64.
#
# NOTE: On Ubuntu 20.04, glibc cannot give OpenMP its thread-local storage once
# Python has loaded other libraries, and the first import of OpenCV fails. Loading
# OpenMP first avoids it. A PyPI wheel can carry an OpenMP of its own
# which fails the same way when imported after OpenCV.
if [ "$(. /etc/os-release 2>/dev/null && echo "$ID-$VERSION_ID")" = "ubuntu-20.04" ] \
    && [ -e "$CONDA_PREFIX/lib/libgomp.so.1" ]; then
    LD_PRELOAD="$CONDA_PREFIX/lib/libgomp.so.1${LD_PRELOAD:+:$LD_PRELOAD}"
    for lib in "$CONDA_PREFIX"/lib/python3.*/site-packages/*.libs/libgomp*.so*; do
        # One of the python3.* directories is a link to the other
        [ -L "${lib%/site-packages/*}" ] && continue
        [ -f "$lib" ] && LD_PRELOAD="$LD_PRELOAD:$lib"
    done
    export LD_PRELOAD
fi
