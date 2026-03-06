# EMOS Jetson Docker Images

These Dockerfiles are for **manual builds on NVIDIA Jetson hardware** (JetPack/L4T). They are NOT built in CI because they require Jetson-specific base images and NVIDIA runtime.

## Prerequisites

- NVIDIA Jetson device (Orin, Xavier, etc.) with JetPack installed
- Docker with NVIDIA Container Runtime configured as default runtime
- Build context must be the **repository root** (not this directory)

## Building

From the repository root:

```bash
docker build \
  -f docker/jetson/Dockerfile.jazzy \
  -t emos:jetson-jazzy \
  .
```

## Key Differences from Multi-Arch Image

- Based on `dustynv/onnxruntime` (L4T Ubuntu 20.04) instead of official ROS images
- Builds ROS 2 from source (official images don't support L4T)
- Installs `kompass-core` with GPU acceleration via CUDA/TensorRT
- Single-stage build (no multi-stage, since the base image is already large)
- arm64 only (Jetson is always aarch64)

## Notes

- The default Docker runtime must be set to `nvidia` for the GPU verification step
- The Python 3.8 compatibility patch is applied automatically for Jazzy on L4T
- Sensor drivers are NOT included; mount them as separate containers
