#!/bin/bash
# install_mapping_backend_pixi.sh — Build the native mapping backend (GLIM) inside a pixi environment.
#
# GTSAM, gtsam_points, GLIM and glim_ros2 are built from source against the libraries the
# environment already has (Boost, Eigen, fmt, spdlog, OpenCV, ROS)

# Run from inside the pixi environment (pixi run install-mapping-backend)
# EMOS_MAPPING_CUDA, set by 'emos map setup', is the CUDA toolkit to build GLIM's CUDA modules
# with. Unset builds for the CPU alone.

set -eo pipefail

GTSAM_REPO="https://github.com/borglab/gtsam"
GTSAM_REF="4.3a1"
GTSAM_POINTS_REPO="https://github.com/koide3/gtsam_points"
GTSAM_POINTS_REF="v1.2.2"
# v1.2.2 plus the build fix for fmt >= 11
GLIM_REPO="https://github.com/aleph-ra/glim"
GLIM_REF="v1.2.2-emos1"
# v1.2.2 plus the fix for the map publisher reading past its submaps
GLIM_ROS_REPO="https://github.com/koide3/glim_ros2"
GLIM_ROS_REF="4d4ec524ccf1b02aa09b0af2af767ecc54343798"

CUDA_ROOT="${EMOS_MAPPING_CUDA:-}"

ROOT="${PIXI_PROJECT_ROOT:-$(pwd)}"
WORK="$ROOT/mapping_backend" # sources, build trees and logs
INSTALL="$ROOT/install"      # the workspace recipes run from

log() {
    local level="$1" message="$2"
    local ts=$(date +"%Y-%m-%d %H:%M:%S")
    case "$level" in
        INFO)  echo -e "\033[1;34m[$ts] [INFO]\033[0m $message" ;;
        ERROR) echo -e "\033[1;31m[$ts] [ERROR]\033[0m $message" >&2 ;;
    esac
}

# fetch <name> <repo> <ref>: check out exactly ref, a tag or a commit.
fetch() {
    local dir="$WORK/src/$1"
    local pinned="$dir/.git/emos_ref"
    if [ -f "$pinned" ] && [ "$(cat "$pinned")" = "$3" ]; then
        return 0
    fi
    if [ ! -d "$dir/.git" ]; then
        git init -q "$dir"
        git -C "$dir" remote add origin "$2"
    fi
    git -C "$dir" fetch -q --depth 1 origin "$3"
    git -C "$dir" checkout -q --detach FETCH_HEAD
    echo "$3" > "$pinned"
}

# build <name> [cmake args...]: build one source tree into the workspace
build() {
    local name="$1"
    shift
    log INFO "Building $name..."
    colcon --log-base "$WORK/log" build \
        --merge-install --install-base "$INSTALL" --build-base "$WORK/build" \
        --base-paths "$WORK/src/$name" --parallel-workers 1 \
        --cmake-args -DCMAKE_BUILD_TYPE=Release -DCMAKE_POLICY_VERSION_MINIMUM=3.5 "$@"
}

if [ -z "$PIXI_PROJECT_ROOT" ] && [ -z "$CONDA_PREFIX" ]; then
    log ERROR "Run this inside the pixi environment: pixi run install-mapping-backend"
    exit 1
fi

# TODO: On aarch64 the environment adds the system's glibc headers to CFLAGS for
# one PyPI source build. Ahead of the environment's own sysroot they break every
# C source that includes math.h. Needs to be fixed at source (pyaudio build)
export CFLAGS="${CFLAGS//-I\/usr\/include\/aarch64-linux-gnu/}"

CUDA_ARGS=(-DBUILD_WITH_CUDA=OFF)
if [ -n "$CUDA_ROOT" ]; then
    if [ ! -x "$CUDA_ROOT/bin/nvcc" ]; then
        log ERROR "No CUDA compiler at $CUDA_ROOT/bin/nvcc"
        exit 1
    fi
    # gtsam_points builds for the GPU it finds on the board.
    # The CUDA libraries use the system's glibc, newer than the sysroot the linker
    # checks against.
    CUDA_ARGS=(-DBUILD_WITH_CUDA=ON -DCMAKE_CUDA_COMPILER="$CUDA_ROOT/bin/nvcc"
        -DCUDAToolkit_ROOT="$CUDA_ROOT" -DCMAKE_CUDA_HOST_COMPILER=/usr/bin/g++
        -DCMAKE_EXE_LINKER_FLAGS=-Wl,--allow-shlib-undefined)
fi

export MAKEFLAGS="-j$(nproc)"
log INFO "Building the mapping backend in $WORK with $MAKEFLAGS${CUDA_ROOT:+, with CUDA from $CUDA_ROOT}"
mkdir -p "$WORK/src"

fetch gtsam "$GTSAM_REPO" "$GTSAM_REF"
fetch gtsam_points "$GTSAM_POINTS_REPO" "$GTSAM_POINTS_REF"
fetch glim "$GLIM_REPO" "$GLIM_REF"
fetch glim_ros2 "$GLIM_ROS_REPO" "$GLIM_ROS_REF"

# Each one needs the one before it installed
build gtsam -DGTSAM_BUILD_EXAMPLES_ALWAYS=OFF -DGTSAM_BUILD_TESTS=OFF -DGTSAM_WITH_TBB=OFF \
    -DGTSAM_BUILD_WITH_MARCH_NATIVE=OFF -DGTSAM_USE_SYSTEM_EIGEN=ON
build gtsam_points "${CUDA_ARGS[@]}" -DBUILD_WITH_MARCH_NATIVE=OFF
# Turn off OpenCV
build glim "${CUDA_ARGS[@]}" -DBUILD_WITH_VIEWER=OFF -DBUILD_WITH_OPENCV=OFF -DBUILD_WITH_MARCH_NATIVE=OFF
build glim_ros2 "${CUDA_ARGS[@]}" -DBUILD_WITH_VIEWER=OFF -DBUILD_WITH_CV_BRIDGE=OFF

log INFO "Mapping backend installed into $INSTALL"
