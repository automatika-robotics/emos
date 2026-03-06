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
    nlohmann-json3-dev \
    libgeographic-dev \
    && rm -rf /var/lib/apt/lists/*

# Python dependencies
RUN pip install --no-cache-dir --break-system-packages \
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
    'fastapi[standard-no-fastapi-cloud-cli]' \
    kompass-core

# --- Build rmw_zenoh from source ---
WORKDIR ${ROS_WS}
RUN curl https://sh.rustup.rs -sSf | sh -s -- -y
ENV PATH="/root/.cargo/bin:${PATH}"
RUN mkdir -p src \
    && git clone --depth 1 -b ${ROS_DISTRO} https://github.com/ros2/rmw_zenoh.git src/rmw_zenoh \
    && /bin/bash -c "source ${ROS_ROOT}/setup.bash && \
       colcon build --merge-install --install-base ${ROS_ROOT} \
       --cmake-args -DCMAKE_BUILD_TYPE=Release" \
    && rm -rf ${ROS_WS}/src ${ROS_WS}/build ${ROS_WS}/log \
    && rm -rf /root/.cargo /root/.rustup \
    && sed -i '/cargo\/env/d' /root/.bashrc

# --- Build localization packages from source ---
WORKDIR ${ROS_WS}/src
RUN git clone --depth 1 -b ${ROS_DISTRO} https://github.com/ros-geographic-info/geographic_info.git \
    && git clone --depth 1 -b ${ROS_DISTRO}-devel https://github.com/cra-ros-pkg/robot_localization.git \
    && git clone --depth 1 -b ros2 https://github.com/ros/angles.git \
    && rm -rf geographic_info/geographic_info geographic_info/geodesy

WORKDIR ${ROS_WS}
RUN /bin/bash -c "source ${ROS_ROOT}/setup.bash && \
    colcon build --merge-install --install-base ${ROS_ROOT} \
    --cmake-args -DCMAKE_BUILD_TYPE=Release" \
    && rm -rf ${ROS_WS}/src ${ROS_WS}/build ${ROS_WS}/log

# --- Build EMOS packages ---
WORKDIR ${ROS_WS}/src
COPY stack/sugarcoat sugarcoat
COPY stack/kompass kompass
COPY stack/embodied-agents embodied-agents

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
ENV RMW_IMPLEMENTATION=rmw_zenoh_cpp

# Runtime-only system dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    portaudio19-dev \
    libgeographic-dev \
    jq \
    && rm -rf /var/lib/apt/lists/*

# Copy built ROS install space from builder
COPY --from=builder ${ROS_ROOT}/ ${ROS_ROOT}/

# Copy Python site-packages from builder
COPY --from=builder /usr/local/lib/python3*/dist-packages/ /usr/local/lib/python3/dist-packages/
COPY --from=builder /usr/lib/python3/dist-packages/ /usr/lib/python3/dist-packages/

# Entrypoint
COPY docker/emos/entrypoint.sh /ros_entrypoint.sh
RUN chmod +x /ros_entrypoint.sh

ENTRYPOINT ["/ros_entrypoint.sh"]
CMD ["bash"]
