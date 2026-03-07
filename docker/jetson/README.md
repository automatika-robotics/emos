# EMOS Jetson Docker Images

These Dockerfiles are for **manual builds on NVIDIA Jetson hardware** (JetPack/L4T). They are NOT built in CI because they require Jetson-specific base images and NVIDIA runtime.

## Prerequisites

- NVIDIA Jetson device with JetPack installed
- Docker with NVIDIA Container Runtime configured as default runtime
- Build context must be the **repository root** (not this directory)

## Base Images

Choose the base image matching your Jetson platform:

| Jetson Platform | JetPack | L4T | Base Image (`JETSON_BASE`) |
|---|---|---|---|
| Xavier (AGX/NX) | 5.x | R35.x | `dustynv/onnxruntime:r35.4.1` |
| Orin (AGX/NX/Nano) | 6.x | R36.x | `dustynv/onnxruntime:1.22-r36.4.0-cu128-24.04` |
| Thor | 7.x | R38.x | Not yet available (JetPack 7 containers pending) |

All base images include ONNX Runtime with GPU support for running local inference models in EmbodiedAgents.

## Building

From the repository root, specify `JETSON_BASE` and `ROS_DISTRO`:

```bash
# Orin + Jazzy (default)
docker build -f docker/jetson/Dockerfile.jetson \
  -t emos:jetson-orin-jazzy .

# Orin + Humble
docker build -f docker/jetson/Dockerfile.jetson \
  --build-arg ROS_DISTRO=humble \
  -t emos:jetson-orin-humble .

# Orin + Kilted
docker build -f docker/jetson/Dockerfile.jetson \
  --build-arg ROS_DISTRO=kilted \
  -t emos:jetson-orin-kilted .

# Xavier + Jazzy
docker build -f docker/jetson/Dockerfile.jetson \
  --build-arg JETSON_BASE=dustynv/onnxruntime:r35.4.1 \
  --build-arg ROS_DISTRO=jazzy \
  -t emos:jetson-xavier-jazzy .

# Xavier + Humble
docker build -f docker/jetson/Dockerfile.jetson \
  --build-arg JETSON_BASE=dustynv/onnxruntime:r35.4.1 \
  --build-arg ROS_DISTRO=humble \
  -t emos:jetson-xavier-humble .
```

## Key Differences from Multi-Arch Image

- Based on `dustynv/onnxruntime` (L4T) instead of official ROS images
- Builds ROS 2 from source (official images don't support L4T)
- Installs `kompass-core` with GPU acceleration via CUDA/TensorRT
- Builds `rmw_zenoh` from source (no apt packages for L4T)
- Single-stage build (no multi-stage, since the base image is already large)
- arm64 only (Jetson is always aarch64)

## Notes

- The default Docker runtime must be set to `nvidia` for the GPU verification step
- The Python 3.8 compatibility patch is applied automatically when needed (Xavier/L4T R35 base is Ubuntu 20.04)
- Sensor drivers are NOT included; mount them as separate containers
- Check your JetPack version with `cat /etc/nv_tegra_release` or `dpkg -l nvidia-l4t-core`
