#!/bin/bash
# install_kompass_core_pixi.sh — Install kompass-core with GPU (SYCL) support inside a pixi environment.
#
# This script installs only what pixi/conda-forge can't provide:
#   1. LLVM/Clang and the OpenCL headers/loader (via apt — needed to build AdaptiveCpp)
#   2. AdaptiveCpp (built from source against the system toolchain)
#   3. kompass-core (built from source against pixi's OMPL, FCL, Boost)
#
# Prerequisites: run from inside pixi environment (pixi run bash install_kompass_core_pixi.sh)
# OMPL, FCL, Boost (libboost-devel), ODE must be in pixi.toml dependencies.

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

# Run commands outside the pixi environment: pixi's LD_LIBRARY_PATH breaks apt/dpkg, and
# AdaptiveCpp must be configured with the system CMake and system libraries. The
# environment's CMake searches the environment first (on linux-64 it contains its own
# OpenCL loader, which does not see the system's OpenCL drivers).
clean_env() {
    env -u LD_LIBRARY_PATH -u CONDA_PREFIX -u PIXI_PROJECT_MANIFEST \
        -u CC -u CXX -u CFLAGS -u CXXFLAGS -u CPPFLAGS -u LDFLAGS -u CMAKE_ARGS -u CMAKE_PREFIX_PATH \
        PATH="/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin" "$@"
}

# Version of a conda package installed in the pixi environment (empty if absent)
env_pkg_version() {
    ls "$PIXI_PREFIX/conda-meta" 2>/dev/null | sed -n "s/^$1-\([0-9][0-9.]*\)-[^-]*\.json$/\1/p" | sed -n 1p
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
    log ERROR "Usage: pixi run bash install_kompass_core_pixi.sh"
    exit 1
fi
log INFO "pixi environment: $PIXI_PREFIX"

# Verify OMPL/FCL from pixi (OMPL uses versioned include dir e.g. include/ompl-1.7/)
OMPL_HEADER=$(find "$PIXI_PREFIX/include" -maxdepth 2 -type d -name "ompl" -print -quit 2>/dev/null)
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

# Boost headers and CMake config must come from the pixi env and match the Boost runtime
BOOST_VERSION=$(env_pkg_version libboost)
BOOST_DEVEL_VERSION=$(env_pkg_version libboost-devel)
if [[ -z "$BOOST_VERSION" ]]; then
    log ERROR "Boost not found in pixi env. Add 'libboost-devel' to pixi.toml and run 'pixi install'."
    exit 1
fi
if [[ "$BOOST_DEVEL_VERSION" != "$BOOST_VERSION" ]]; then
    log ERROR "pixi env has Boost $BOOST_VERSION but libboost-devel ${BOOST_DEVEL_VERSION:-is missing}. Add 'libboost-devel' to pixi.toml and run 'pixi install'."
    exit 1
fi
log INFO "Boost $BOOST_VERSION found in pixi env"

# pixi's OMPL, FCL and Boost are built against the env's libstdc++, kompass-core links against the env's libstdc++
if [[ ! -e "$PIXI_PREFIX/lib/libstdc++.so" ]]; then
    log ERROR "libstdc++ not found in pixi env. Add 'compilers' to pixi.toml and run 'pixi install'."
    exit 1
fi

# ---- apt: LLVM/Clang, OpenCL headers and loader, build tools ----

log INFO "Installing LLVM/Clang via apt..."
clean_env $SUDO apt update -y
clean_env $SUDO apt install -y lsb-release bc wget gnupg software-properties-common \
    git cmake make ocl-icd-opencl-dev opencl-c-headers

export DEBIAN_FRONTEND=noninteractive
LLVM_VERSION=17

# Check if already installed
FOUND=0
for v in $(seq 14 17); do
    if clean_env llvm-config-$v --version &>/dev/null && clean_env clang++-$v --version &>/dev/null; then
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

LLVM_DIR=$(clean_env llvm-config-${LLVM_VERSION} --cmakedir)
CLANG_PATH=$(clean_env which clang++-${LLVM_VERSION})
CLANG_C_PATH=$(clean_env which clang-${LLVM_VERSION})
log INFO "LLVM cmake: $LLVM_DIR | Clang++: $CLANG_PATH"

# libstdc++
GCC_VER=$(clean_env clang++-${LLVM_VERSION} -v 2>&1 | awk -F/ '/Selected GCC/ {print $NF}' || echo "")
if [[ -n "$GCC_VER" ]]; then
    clean_env $SUDO apt install -y "libstdc++-${GCC_VER}-dev" 2>/dev/null || clean_env $SUDO apt install -y libstdc++-dev
else
    clean_env $SUDO apt install -y libstdc++-dev
fi

# ---- Build AdaptiveCpp ----

# TODO: Switch back to a versioned upstream AdaptiveCpp release once the changes
# needed for Arm Mali (AdaptiveCpp/AdaptiveCpp#2228) are merged upstream.
ACPP_URL="https://github.com/aleph-ra/AdaptiveCpp"
ACPP_VERSION="v25.10.0-mali"
ACPP_PREFIX="/usr/local"
ACPP_STAGE="/tmp/acpp-stage-$$"

log INFO "Building AdaptiveCpp $ACPP_VERSION..."
cd /tmp
rm -rf AdaptiveCpp "$ACPP_STAGE"
mkdir -p "$ACPP_STAGE"
clean_env git clone --depth 1 --branch "$ACPP_VERSION" "$ACPP_URL" AdaptiveCpp
cd AdaptiveCpp && mkdir -p build && cd build

# Configure, build and install with the system CMake from inside the build tree
clean_env cmake -DCMAKE_BUILD_TYPE=Release -DCMAKE_INSTALL_PREFIX="$ACPP_STAGE" \
    -DCMAKE_C_COMPILER="$CLANG_C_PATH" -DCMAKE_CXX_COMPILER="$CLANG_PATH" \
    -DLLVM_DIR="$LLVM_DIR" -DCLANG_EXECUTABLE_PATH="$CLANG_PATH" ..
clean_env make -j$(nproc)
clean_env make install -j$(nproc)

log INFO "Copying AdaptiveCpp artefacts into $ACPP_PREFIX (requires sudo)..."
clean_env $SUDO cp -a "$ACPP_STAGE/." "$ACPP_PREFIX/"
clean_env $SUDO ldconfig

cd /tmp && rm -rf AdaptiveCpp "$ACPP_STAGE"

if clean_env acpp --acpp-version &>/dev/null; then
    log INFO "AdaptiveCpp installed successfully."
else
    log ERROR "AdaptiveCpp installation failed."
    exit 1
fi
# Without the translator every OpenCL kernel fails at JIT time
if [[ -f "$ACPP_PREFIX/lib/hipSYCL/librt-backend-ocl.so" && ! -x "$ACPP_PREFIX/lib/hipSYCL/ext/llvm-spirv/bin/llvm-spirv" ]]; then
    log ERROR "AdaptiveCpp's OpenCL backend is installed but its llvm-spirv translator is missing."
    exit 1
fi

# ---- Build kompass-core ----

log INFO "Building kompass-core..."
cd /tmp
rm -rf kompass-core

KC_TAG=$(curl -s "https://api.github.com/repos/automatika-robotics/kompass-core/tags" | grep -o '"name": "[^"]*"' | sed -n 1p | cut -d'"' -f4)
log INFO "Cloning kompass-core $KC_TAG..."
git clone --depth 1 --branch "$KC_TAG" https://github.com/automatika-robotics/kompass-core
cd kompass-core

python3 -m pip install "scikit-build-core>=0.8" "nanobind>=1.8,<2.9.2" "packaging>=22.0"
python3 -m pip uninstall -y kompass-core 2>/dev/null || true

# Point cmake at pixi's OMPL, FCL, Boost and the system AdaptiveCpp
export CMAKE_PREFIX_PATH="$PIXI_PREFIX:$ACPP_PREFIX${CMAKE_PREFIX_PATH:+:$CMAKE_PREFIX_PATH}"
log INFO "CMAKE_PREFIX_PATH=$CMAKE_PREFIX_PATH"

# Link against the env's libstdc++ and let the installed modules find it at runtime
export LDFLAGS="-L$PIXI_PREFIX/lib${LDFLAGS:+ $LDFLAGS}"
export SKBUILD_CMAKE_ARGS="-DCMAKE_INSTALL_RPATH=$PIXI_PREFIX/lib${SKBUILD_CMAKE_ARGS:+;$SKBUILD_CMAKE_ARGS}"

export CMAKE_BUILD_PARALLEL_LEVEL="$(nproc)"
CXX=$CLANG_PATH python3 -m pip install --no-build-isolation .

cd /tmp && rm -rf kompass-core

# ---- Verify ----

INSTALL_OK=true
python3 -c "import kompass_cpp" 2>/dev/null || { log ERROR "Failed to import kompass_cpp."; INSTALL_OK=false; }
python3 -c "import omplpy" 2>/dev/null   || { log ERROR "Failed to import omplpy."; INSTALL_OK=false; }

# kompass_cpp must use the env's Boost, not a system Boost
KC_MODULE=$(python3 -c "import kompass_cpp; print(kompass_cpp.__file__)" 2>/dev/null || true)
if [[ -n "$KC_MODULE" ]] && command -v readelf &>/dev/null; then
    KC_BOOST=$(readelf -d "$KC_MODULE" | sed -n 's/.*libboost_filesystem\.so\.\([0-9.]*\).*/\1/p' | sed -n 1p)
    if [[ -n "$KC_BOOST" && "$KC_BOOST" != "$BOOST_VERSION" ]]; then
        log ERROR "kompass_cpp links Boost $KC_BOOST instead of the pixi env's Boost $BOOST_VERSION."
        INSTALL_OK=false
    fi
fi

if [ "$INSTALL_OK" = true ]; then
    log INFO "\033[1;32mkompass-core installed successfully (pixi mode).\033[0m"
else
    log ERROR "kompass-core installation failed."
    exit 1
fi

# Explicit end-of-script marker
log INFO "\033[1;32m✓ install_kompass_core_pixi.sh: all steps completed.\033[0m"
