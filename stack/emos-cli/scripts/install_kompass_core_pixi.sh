#!/bin/bash
# install_gpu_pixi.sh — Install kompass-core with GPU (SYCL) support inside a pixi environment.
#
# This script installs only what pixi/conda-forge can't provide:
#   1. LLVM/Clang (via apt — needed for AdaptiveCpp/SYCL compiler)
#   2. AdaptiveCpp (built from source)
#   3. kompass-core (built from source against pixi's OMPL, FCL, Boost)
#
# Prerequisites: run from inside pixi environment (pixi run bash install_gpu_pixi.sh)
# OMPL, FCL, Boost, ODE must be in pixi.toml dependencies.

set -eo pipefail

log() {
    local level="$1" message="$2"
    local ts=$(date +"%Y-%m-%d %H:%M:%S")
    case "$level" in
        INFO)  echo -e "\033[1;34m[$ts] [INFO]\033[0m $message" ;;
        WARN)  echo -e "\033[1;33m[$ts] [WARN]\033[0m $message" >&2 ;;
        ERROR) echo -e "\033[1;31m[$ts] [ERROR]\033[0m $message" >&2 ;;
    esac
}

is_in_container() {
    [[ -f /.dockerenv ]] || grep -qE '(docker|podman|containerd)' /proc/self/cgroup 2>/dev/null || [[ -f /run/.containerenv ]]
}

# Run commands with sanitized env so pixi's LD_LIBRARY_PATH doesn't break apt/dpkg
clean_env() {
    env -u LD_LIBRARY_PATH -u CONDA_PREFIX -u PIXI_PROJECT_MANIFEST "$@"
}

SUDO=$(is_in_container && echo "" || echo "sudo")

# Check sudo
if [[ -n "$SUDO" ]] && ! sudo -n true 2>/dev/null; then
    log WARN "This script requires sudo privileges."
    sudo -v || { log ERROR "Failed to acquire sudo privileges."; exit 1; }
fi

# Verify we are inside pixi
PIXI_PREFIX="${CONDA_PREFIX:-}"
if [[ -z "$PIXI_PREFIX" ]]; then
    log ERROR "Not running inside a pixi environment."
    log ERROR "Usage: pixi run bash install_gpu_pixi.sh"
    exit 1
fi
log INFO "pixi environment: $PIXI_PREFIX"

# Verify OMPL/FCL from pixi (OMPL uses versioned include dir e.g. include/ompl-1.7/)
OMPL_HEADER=$(find "$PIXI_PREFIX/include" -maxdepth 2 -type d -name "ompl" 2>/dev/null | head -1)
FCL_HEADER="$PIXI_PREFIX/include/fcl"
if [[ -z "$OMPL_HEADER" ]]; then
    log ERROR "OMPL not found in pixi env. Add 'ompl' to pixi.toml and run 'pixi install'."
    exit 1
fi
if [[ ! -d "$FCL_HEADER" ]]; then
    log ERROR "FCL not found in pixi env. Add 'fcl' to pixi.toml and run 'pixi install'."
    exit 1
fi
log INFO "OMPL found at $OMPL_HEADER"
log INFO "FCL found at $FCL_HEADER"

# ---- apt: LLVM/Clang only ----

log INFO "Installing LLVM/Clang via apt..."
clean_env $SUDO apt update -y
clean_env $SUDO apt install -y lsb-release bc wget gnupg software-properties-common

export DEBIAN_FRONTEND=noninteractive
LLVM_VERSION=17

# Check if already installed
FOUND=0
for v in $(seq 14 17); do
    if llvm-config-$v --version &>/dev/null && clang++-$v --version &>/dev/null; then
        FOUND=$v
    fi
done

if [[ $FOUND -eq 0 ]]; then
    if clean_env $SUDO apt install -y "llvm-$LLVM_VERSION" "clang-$LLVM_VERSION" "libclang-$LLVM_VERSION-dev" "libomp-$LLVM_VERSION-dev" "lld-$LLVM_VERSION" 2>/dev/null; then
        log INFO "Installed LLVM/Clang $LLVM_VERSION from apt."
    else
        log INFO "Falling back to llvm.sh..."
        wget -q https://apt.llvm.org/llvm.sh && chmod +x llvm.sh
        clean_env $SUDO ./llvm.sh "$LLVM_VERSION"
        rm -f llvm.sh
    fi
else
    LLVM_VERSION=$FOUND
    log INFO "Found existing LLVM/Clang $FOUND."
fi

clean_env $SUDO apt install -y \
    "libclang-${LLVM_VERSION}-dev" "clang-tools-${LLVM_VERSION}" \
    "libomp-${LLVM_VERSION}-dev" "llvm-${LLVM_VERSION}-dev" "lld-${LLVM_VERSION}"

LLVM_DIR=$(llvm-config-${LLVM_VERSION} --cmakedir)
CLANG_PATH=$(which clang++-${LLVM_VERSION})
log INFO "LLVM cmake: $LLVM_DIR | Clang++: $CLANG_PATH"

# libstdc++
GCC_VER=$(clang++-${LLVM_VERSION} -v 2>&1 | awk -F/ '/Selected GCC/ {print $NF}' || echo "")
if [[ -n "$GCC_VER" ]]; then
    clean_env $SUDO apt install -y "libstdc++-${GCC_VER}-dev" 2>/dev/null || clean_env $SUDO apt install -y libstdc++-dev
else
    clean_env $SUDO apt install -y libstdc++-dev
fi

# ---- Build AdaptiveCpp ----

ACPP_VERSION="v25.10.0"
ACPP_PREFIX="/usr/local"
ACPP_STAGE="/tmp/acpp-stage-$$"

log INFO "Building AdaptiveCpp $ACPP_VERSION..."
cd /tmp
rm -rf AdaptiveCpp "$ACPP_STAGE"
mkdir -p "$ACPP_STAGE"
git clone --depth 1 --branch "$ACPP_VERSION" https://github.com/AdaptiveCpp/AdaptiveCpp
cd AdaptiveCpp && mkdir -p build && cd build

CMAKE_FLAGS="-DCMAKE_INSTALL_PREFIX=$ACPP_STAGE -DLLVM_DIR=$LLVM_DIR -DCLANG_EXECUTABLE_PATH=$CLANG_PATH"
CXX=$CLANG_PATH clean_env cmake $CMAKE_FLAGS ..
clean_env make -j$(nproc)
clean_env make install -j$(nproc)

log INFO "Copying AdaptiveCpp artefacts into $ACPP_PREFIX (requires sudo)..."
clean_env $SUDO cp -a "$ACPP_STAGE/." "$ACPP_PREFIX/"

cd /tmp && rm -rf AdaptiveCpp "$ACPP_STAGE"

if acpp --acpp-version &>/dev/null; then
    log INFO "AdaptiveCpp installed successfully."
else
    log ERROR "AdaptiveCpp installation failed."
    exit 1
fi

# ---- Build kompass-core ----

log INFO "Building kompass-core..."
cd /tmp
rm -rf kompass-core

LATEST_TAG=$(curl -s "https://api.github.com/repos/automatika-robotics/kompass-core/tags" | grep -o '"name": "[^"]*"' | head -1 | cut -d'"' -f4)
log INFO "Cloning kompass-core at tag $LATEST_TAG..."
git clone --depth 1 --branch "$LATEST_TAG" https://github.com/automatika-robotics/kompass-core
cd kompass-core

python3 -m pip install "scikit-build-core>=0.8" "nanobind>=1.8,<2.9.2" "packaging>=22.0"
python3 -m pip uninstall -y kompass-core 2>/dev/null || true

# Point cmake at pixi's OMPL, FCL, Boost
export CMAKE_PREFIX_PATH="$PIXI_PREFIX:${CMAKE_PREFIX_PATH:-}"
log INFO "CMAKE_PREFIX_PATH=$CMAKE_PREFIX_PATH"

CXX=$CLANG_PATH python3 -m pip install --no-build-isolation .

cd /tmp && rm -rf kompass-core

# ---- Verify ----

INSTALL_OK=true
python3 -c "import kompass_cpp" 2>/dev/null || { log ERROR "Failed to import kompass_cpp."; INSTALL_OK=false; }
python3 -c "import omplpy" 2>/dev/null   || { log ERROR "Failed to import omplpy."; INSTALL_OK=false; }

if [ "$INSTALL_OK" = true ]; then
    log INFO "\033[1;32mkompass-core installed successfully (pixi mode).\033[0m"
else
    log ERROR "kompass-core installation failed."
    exit 1
fi

# Explicit end-of-script marker
log INFO "\033[1;32m✓ install_kompass_core_pixi.sh: all steps completed.\033[0m"
