# EMOS Multi-Arch Dockerfile
# Build: docker build -f docker/emos/Dockerfile --build-arg ROS_DISTRO=jazzy -t emos:test .
# Must be run from the repo root so COPY stack/... works.

ARG ROS_DISTRO=jazzy

# =============================================================================
# Stage 1: Builder
# =============================================================================
FROM ros:${ROS_DISTRO}-ros-base AS builder

ARG ROS_DISTRO
ENV ROS_DISTRO=${ROS_DISTRO}
ENV ROS_ROOT=/opt/ros/${ROS_DISTRO}
ENV ROS_WS=/tmp/ros2_ws
ENV DEBIAN_FRONTEND=noninteractive

# System dependencies for building
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    cmake \
    git \
    curl \
    python3-pip \
    portaudio19-dev \
    jq \
    ros-${ROS_DISTRO}-rmw-zenoh-cpp \
    && rm -rf /var/lib/apt/lists/*

# Python dependencies (excluding kompass-core, installed via GPU script below)
ENV PIP_BREAK_SYSTEM_PACKAGES=1
RUN pip install --no-cache-dir --ignore-installed \
    numpy \
    opencv-python-headless \
    'attrs>=23.2.0' \
    jinja2 \
    httpx \
    setproctitle \
    msgpack \
    msgpack-numpy \
    platformdirs \
    tqdm \
    pyyaml \
    toml \
    websockets \
    ollama \
    'redis[hiredis]' \
    pyaudio \
    soundfile \
    'python-fasthtml>=0.12,<0.15' \
    'monsterui>=1.0,<1.1' \
    huggingface-hub \
    sherpa-onnx \
    onnxruntime \
    grpcio \
    protobuf

# Install llama-cpp-python from its own index. --only-binary takes the newest
# wheel there.
RUN pip install --no-cache-dir --only-binary=llama-cpp-python \
    --extra-index-url https://abetlen.github.io/llama-cpp-python/whl/cpu llama-cpp-python

# Install kompass-core with GPU support (also works on non-GPU machines)
ADD https://raw.githubusercontent.com/automatika-robotics/kompass-core/main/build_dependencies/install_gpu.sh /tmp/install_gpu.sh
RUN chmod +x /tmp/install_gpu.sh && /tmp/install_gpu.sh && rm /tmp/install_gpu.sh

# --- Build EMOS packages ---
WORKDIR ${ROS_WS}/src
COPY stack/sugarcoat sugarcoat
COPY stack/kompass kompass
COPY stack/embodied-agents embodied-agents
COPY stack/emos_mapping emos_mapping

WORKDIR ${ROS_WS}
RUN /bin/bash -c "source ${ROS_ROOT}/setup.bash && \
    colcon build --merge-install --install-base ${ROS_ROOT} \
    --cmake-args -DCMAKE_BUILD_TYPE=Release" \
    && rm -rf ${ROS_WS} /root/.ros /tmp/*

# =============================================================================
# Stage 2: Final
# =============================================================================
FROM ros:${ROS_DISTRO}-ros-base

ARG ROS_DISTRO
ENV ROS_DISTRO=${ROS_DISTRO}
ENV ROS_ROOT=/opt/ros/${ROS_DISTRO}
ENV DEBIAN_FRONTEND=noninteractive

# Runtime-only system dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    portaudio19-dev \
    jq \
    ros-${ROS_DISTRO}-rmw-zenoh-cpp \
    libomp-dev \
    && rm -rf /var/lib/apt/lists/*

# Copy built ROS install space from builder
COPY --from=builder ${ROS_ROOT}/ ${ROS_ROOT}/

# Copy AdaptiveCpp and kompass-core artifacts from builder
COPY --from=builder /usr/local/ /usr/local/

# Copy Python site-packages from builder
COPY --from=builder /usr/lib/python3/dist-packages/ /usr/lib/python3/dist-packages/

# Entrypoint
COPY docker/emos/entrypoint.sh /ros_entrypoint.sh
RUN chmod +x /ros_entrypoint.sh

ENTRYPOINT ["/ros_entrypoint.sh"]
CMD ["bash"]
