#!/bin/bash
# install_cuda_packages_pixi.sh — Build sherpa-onnx and llama-cpp-python with CUDA inside a pixi environment.
#
# Run from inside the pixi environment (pixi run install-cuda-packages)
# EMOS_CUDA, set by 'emos install', is the CUDA toolkit to build with.

set -eo pipefail

ROOT="${PIXI_PROJECT_ROOT:-$(pwd)}"
WORK="$ROOT/cuda_packages" # sources and build trees, removed at the end
WHEELS="$ROOT/cuda_wheels"
CUDA_ROOT="${EMOS_CUDA:-}"

log() {
    local level="$1" message="$2"
    local ts=$(date +"%Y-%m-%d %H:%M:%S")
    case "$level" in
        INFO)  echo -e "\033[1;34m[$ts] [INFO]\033[0m $message" ;;
        ERROR) echo -e "\033[1;31m[$ts] [ERROR]\033[0m $message" >&2 ;;
    esac
}

# The version of a package in the environment, without a local part
installed_version() {
    python3 -c "import importlib.metadata as m; print(m.version('$1').split('+')[0])"
}

if [ ! -x "$CUDA_ROOT/bin/nvcc" ]; then
    log ERROR "No CUDA compiler at $CUDA_ROOT/bin/nvcc"
    exit 1
fi
CUDA_MAJOR=$("$CUDA_ROOT/bin/nvcc" --version | sed -n 's/.*release \([0-9]*\)\..*/\1/p')

# The CUDA libraries use the system's glibc, newer than the sysroot the linker checks against.
LINK_ARGS="-DCMAKE_EXE_LINKER_FLAGS=-Wl,--allow-shlib-undefined -DCMAKE_SHARED_LINKER_FLAGS=-Wl,--allow-shlib-undefined"

rm -rf "$WORK" "$WHEELS"
mkdir -p "$WORK" "$WHEELS"

# --- sherpa-onnx -------------------------------------------------------------
# No CUDA code of its own, downloads prebuilt onnxruntime.
SHERPA_VERSION=$(installed_version sherpa-onnx)
SHERPA_ARGS="-DCMAKE_BUILD_TYPE=Release -DSHERPA_ONNX_ENABLE_GPU=ON -DCMAKE_POLICY_VERSION_MINIMUM=3.5 $LINK_ARGS"
if [ "$(uname -m)" = "aarch64" ]; then
    # The onnxruntime builds its maintainers host for Jetsons, by CUDA version
    case "$CUDA_MAJOR" in
        11) SHERPA_ARGS="$SHERPA_ARGS -DSHERPA_ONNX_LINUX_ARM64_GPU_ONNXRUNTIME_VERSION=1.16.0" ;;
        12) SHERPA_ARGS="$SHERPA_ARGS -DSHERPA_ONNX_LINUX_ARM64_GPU_ONNXRUNTIME_VERSION=1.18.1" ;;
        *)
            log ERROR "sherpa-onnx has no onnxruntime for CUDA $CUDA_MAJOR on aarch64"
            exit 1
            ;;
    esac
fi
log INFO "Building sherpa-onnx $SHERPA_VERSION with CUDA $CUDA_MAJOR..."
git clone -q --depth 1 -b "v$SHERPA_VERSION" https://github.com/k2-fsa/sherpa-onnx "$WORK/sherpa-onnx"
(
    cd "$WORK/sherpa-onnx"
    export SHERPA_ONNX_CMAKE_ARGS="$SHERPA_ARGS" SHERPA_ONNX_MAKE_ARGS="-j$(nproc)"
    python3 -m pip wheel . --no-deps --no-build-isolation -w "$WHEELS"
)

# --- llama-cpp-python --------------------------------------------------------
LLAMA_VERSION=$(installed_version llama-cpp-python)
log INFO "Building llama-cpp-python $LLAMA_VERSION with CUDA $CUDA_MAJOR..."
# nvcc compiles through the system's gcc. llama.cpp builds for the GPU it finds on the board.
export CMAKE_ARGS="-DGGML_CUDA=on -DCMAKE_CUDA_COMPILER=$CUDA_ROOT/bin/nvcc -DCMAKE_CUDA_HOST_COMPILER=/usr/bin/g++ -DCMAKE_CUDA_ARCHITECTURES=native $LINK_ARGS"
export CMAKE_BUILD_PARALLEL_LEVEL="$(nproc)"
python3 -m pip wheel "llama-cpp-python==$LLAMA_VERSION" --no-binary llama-cpp-python --no-deps --no-cache-dir -w "$WHEELS"

rm -rf "$WORK"
log INFO "CUDA wheels are in $WHEELS"
