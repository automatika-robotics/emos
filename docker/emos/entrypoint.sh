#!/bin/bash
set -e

# Source the ROS 2 installation
if [ -f "/opt/ros/${ROS_DISTRO}/setup.bash" ]; then
  source "/opt/ros/${ROS_DISTRO}/setup.bash"
fi

# Default middleware to Zenoh
export RMW_IMPLEMENTATION="${RMW_IMPLEMENTATION:-rmw_zenoh_cpp}"

# Source user workspace overlay if present
if [ -f "/emos/workspace/install/setup.bash" ]; then
  source "/emos/workspace/install/setup.bash"
fi

exec "$@"
