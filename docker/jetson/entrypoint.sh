#!/bin/bash
set -e

# Source the ROS 2 installation
if [ -f "/opt/ros/${ROS_DISTRO}/setup.bash" ]; then
  source "/opt/ros/${ROS_DISTRO}/setup.bash"
  export LD_LIBRARY_PATH=/kompass-core/.vcpkg/installed/arm64-linux-release/lib:$LD_LIBRARY_PATH
fi

# Source user workspace overlay if present
if [ -f "/emos/workspace/install/setup.bash" ]; then
  source "/emos/workspace/install/setup.bash"
fi

exec "$@"
