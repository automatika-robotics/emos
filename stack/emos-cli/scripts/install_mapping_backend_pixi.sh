#!/bin/bash
# install_mapping_backend_pixi.sh — Build the native mapping backend (GLIM) inside a pixi environment.
#
# GTSAM, gtsam_points, GLIM and glim_ros2 are built from source against the libraries the
# environment already has (Boost, Eigen, fmt, spdlog, OpenCV, ROS)

# Run from inside the pixi environment (pixi run install-mapping-backend)

set -eo pipefail

GTSAM_REPO="https://github.com/borglab/gtsam"
GTSAM_REF="4.3a1"
GTSAM_POINTS_REPO="https://github.com/koide3/gtsam_points"
GTSAM_POINTS_REF="v1.2.2"
# v1.2.2 plus the build fix for fmt >= 11
GLIM_REPO="https://github.com/aleph-ra/glim"
GLIM_REF="1f87e0744b9e2c08c27ac88a5ffb824a26fdd323"
# v1.2.2 plus the fix for the map publisher reading past its submaps
GLIM_ROS_REPO="https://github.com/koide3/glim_ros2"
GLIM_ROS_REF="4d4ec524ccf1b02aa09b0af2af767ecc54343798"

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

# One compile job per 2 GB of free memory.
jobs() {
    local by_memory=$(awk '/MemAvailable/ {print int($2 / 1024 / 1024 / 2)}' /proc/meminfo)
    local n=$(nproc)
    [ "$by_memory" -lt "$n" ] && n=$by_memory
    [ "$n" -lt 1 ] && n=1
    echo "$n"
}

# fetch <name> <repo> <ref>: check out exactly ref, a tag or a commit
fetch() {
    local dir="$WORK/src/$1"
    if [ ! -d "$dir/.git" ]; then
        git init -q "$dir"
        git -C "$dir" remote add origin "$2"
    fi
    git -C "$dir" fetch -q --depth 1 origin "$3"
    git -C "$dir" checkout -q --detach FETCH_HEAD
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

export MAKEFLAGS="-j$(jobs)"
log INFO "Building the mapping backend in $WORK with $MAKEFLAGS"
mkdir -p "$WORK/src"

fetch gtsam "$GTSAM_REPO" "$GTSAM_REF"
fetch gtsam_points "$GTSAM_POINTS_REPO" "$GTSAM_POINTS_REF"
fetch glim "$GLIM_REPO" "$GLIM_REF"
fetch glim_ros2 "$GLIM_ROS_REPO" "$GLIM_ROS_REF"

# Each one needs the one before it installed
build gtsam -DGTSAM_BUILD_EXAMPLES_ALWAYS=OFF -DGTSAM_BUILD_TESTS=OFF -DGTSAM_WITH_TBB=OFF \
    -DGTSAM_BUILD_WITH_MARCH_NATIVE=OFF -DGTSAM_USE_SYSTEM_EIGEN=ON
build gtsam_points -DBUILD_WITH_CUDA=OFF -DBUILD_WITH_MARCH_NATIVE=OFF
build glim -DBUILD_WITH_CUDA=OFF -DBUILD_WITH_VIEWER=OFF -DBUILD_WITH_MARCH_NATIVE=OFF
build glim_ros2 -DBUILD_WITH_CUDA=OFF -DBUILD_WITH_VIEWER=OFF

log INFO "Mapping backend installed into $INSTALL"
