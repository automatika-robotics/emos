#!/bin/bash
set -e

# =========================
# EMOS - RECIPE RUNNER
# =========================

# Get the directory where the script is located
SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &> /dev/null && pwd)

# Source the common library file
source "$SCRIPT_DIR/emos-lib.sh"

# --- Argument Parsing ---
EMOS_ROOT="/emos"
RECIPES_ROOT="/emos/recipes"
RECIPES_DIR="$HOME/emos/recipes"
RECIPE_NAME=""
RMW_IMPLEMENTATION="rmw_zenoh_cpp"

for arg in "$@"; do
  case $arg in
    --recipe_name=*)
      RECIPE_NAME="${arg#*=}"
      ;;
	--rmw_implementation=*)
      RMW_IMPLEMENTATION="${arg#*=}"
      ;;
    *)
      error "Unknown argument: $arg"
      exit 1
      ;;
  esac
done

# Validate RMW implementation
if [[ "$RMW_IMPLEMENTATION" != "rmw_fastrtps_cpp" && \
      "$RMW_IMPLEMENTATION" != "rmw_cyclonedds_cpp" && \
      "$RMW_IMPLEMENTATION" != "rmw_zenoh_cpp" ]]; then
  error "Invalid RMW_IMPLEMENTATION: $RMW_IMPLEMENTATION"
  error "Allowed values: rmw_fastrtps_cpp, rmw_cyclonedds_cpp, rmw_zenoh_cpp"
  exit 1
fi

if [ -z "$RECIPE_NAME" ]; then
    error "Recipe name was not provided. Usage: $0 --recipe_name=<name>"
    exit 1
fi

# --- Configuration & Pre-flight Checks ---
print_header "EMOS - PRE-RECIPE SETUP"

CONTAINER_NAME="emos"
MANIFEST_FILE="${RECIPES_DIR}/${RECIPE_NAME}/manifest.json"
LOG_DIR="${LOG_DIR:-$HOME/emos/logs}"
DATETIME=$(date +"%Y%m%d_%H%M%S")
LOG_FILE="${LOG_FILE:-${LOG_DIR}/${RECIPE_NAME}_${DATETIME}.log}"

log "Recipe Name: $RECIPE_NAME"
log "Running Recipe Using RMW Implementation: $RMW_IMPLEMENTATION"
log "Container Name: $CONTAINER_NAME"
log "Manifest File: $MANIFEST_FILE"

if [[ ! -f "${MANIFEST_FILE}" ]]; then
    error "Manifest file not found at ${MANIFEST_FILE}"
    exit 1
fi

# Extract manifest data
ACTIVATE_AUTONOMOUS_MODE=$(jq -r '.autonomous_mode' "${MANIFEST_FILE}" 2>/dev/null || echo "false")
RUN_WEB_CLIENT=$(jq -r '.web_client' "${MANIFEST_FILE}" 2>/dev/null || echo "false")
readarray -t ROBOT_SENSORS < <(jq -r '.sensors[]' "${MANIFEST_FILE}" 2>/dev/null)

log "Required sensors: ${ROBOT_SENSORS[*]}"
log "Autonomous mode required: $ACTIVATE_AUTONOMOUS_MODE"
log "Web client required: $RUN_WEB_CLIENT"

# --- Host & Container Management ---
print_header "HOST & CONTAINER MANAGEMENT"

log "Killing all ROS processes on host (sudo may prompt for password)..."
sudo pkill -f roslaunch >/dev/null 2>&1 || true
sudo pkill -f roscore >/dev/null 2>&1 || true
sudo pkill -f ros2 >/dev/null 2>&1 || true
sleep 1
success "Terminated host ROS processes."

if [ -z "$(docker ps -a -q -f name=^/${CONTAINER_NAME}$)" ]; then
    error "Container '$CONTAINER_NAME' does not exist! Run EMOS Setup first."
    exit 1
fi

if [ "$(docker ps -q -f name=$CONTAINER_NAME)" ]; then
    run_with_spinner "Stopping existing EMOS container..." "docker stop $CONTAINER_NAME >/dev/null 2>&1" || true
fi

run_with_spinner "Starting EMOS container..." "docker start $CONTAINER_NAME >/dev/null 2>&1" || (error "Failed to start container." && exit 1)


# --- RMW Configuration ---
print_header "RMW CONFIGURATION"

# Set RMW_IMPLEMENTATION in ros_entrypoint.sh
log "Setting RMW_IMPLEMENTATION=$RMW_IMPLEMENTATION inside container $CONTAINER_NAME..."

docker exec "$CONTAINER_NAME" bash -c "
  if grep -q '^export RMW_IMPLEMENTATION=' /ros_entrypoint.sh; then
    # Replace existing line
    sed -i 's|^export RMW_IMPLEMENTATION=.*|export RMW_IMPLEMENTATION=$RMW_IMPLEMENTATION|' /ros_entrypoint.sh
  else
    # Insert after shebang
    sed -i '1a export RMW_IMPLEMENTATION=$RMW_IMPLEMENTATION' /ros_entrypoint.sh
  fi
"

# --- If using zenoh -> RMW Configuration ---
if [[ "$RMW_IMPLEMENTATION" == "rmw_zenoh_cpp" ]]; then

	# Get zenoh config file from recipe manifest
	ZENOH_ROUTER_CONFIG_FILE=$(jq -r '.zenoh_router_config_file' "${MANIFEST_FILE}" 2>/dev/null)
	# If the value is not null or empty, build full path
	if [[ -n "$ZENOH_ROUTER_CONFIG_FILE" && "$ZENOH_ROUTER_CONFIG_FILE" != "null" ]]; then
		ZENOH_ROUTER_CONFIG_URI="$RECIPES_ROOT/$ZENOH_ROUTER_CONFIG_FILE"
		if [[ "$ZENOH_ROUTER_CONFIG_URI" != *.json5 ]]; then
			warn "Zenoh router config file must be a .json5 file, got '$ZENOH_ROUTER_CONFIG_URI' - Proceeding with default configuration"
			ZENOH_ROUTER_CONFIG_URI=""
		else
			# Verify file exists inside the container before proceeding
			if ! docker exec "$CONTAINER_NAME" test -f "$ZENOH_ROUTER_CONFIG_URI"; then
				log "Using Zenoh router config file '$ZENOH_ROUTER_CONFIG_URI'"
			else
				warn "Zenoh router config file '$ZENOH_ROUTER_CONFIG_URI' not found - Proceeding with default configuration"
				ZENOH_ROUTER_CONFIG_URI=""
			fi
		fi
	else
		log "Using default Zenoh router configuration."
		ZENOH_ROUTER_CONFIG_URI=""
	fi
	# Start zenoh router inside container, detached
	run_with_spinner "Starting zenoh router..."\
					 "docker exec -d $CONTAINER_NAME bash -c \
		             'source ros_entrypoint.sh && ros2 run rmw_zenoh_cpp rmw_zenohd'" || exit 1
	# Give it a moment to start
	sleep 2

	# Set Zenoh config in ros_entrypoint.sh
	if [[ -n "$ZENOH_ROUTER_CONFIG_URI" ]]; then
		docker exec "$CONTAINER_NAME" bash -c "
			if grep -q '^export ZENOH_ROUTER_CONFIG_URI=' /ros_entrypoint.sh; then
			# Replace existing line
			sed -i 's|^export ZENOH_ROUTER_CONFIG_URI=.*|export ZENOH_ROUTER_CONFIG_URI=$ZENOH_ROUTER_CONFIG_URI|' /ros_entrypoint.sh
			else
			# Insert after shebang
			sed -i '1a export ZENOH_ROUTER_CONFIG_URI=$ZENOH_ROUTER_CONFIG_URI' /ros_entrypoint.sh
			fi
		"
	else
		# Remove the config file is exists
		docker exec "$CONTAINER_NAME" bash -c "
			sed -i '/^export ZENOH_ROUTER_CONFIG_URI=/d' /ros_entrypoint.sh
		"
	fi
fi


# --- Hardware & Sensor Launch ---
print_header "HARDWARE & SENSOR LAUNCH"

run_with_spinner "Launching robot base hardware..." \
    "docker exec -d $CONTAINER_NAME bash -c 'source ros_entrypoint.sh && ros2 launch $EMOS_ROOT/robot/launch/bringup_robot.py'" || exit 1

for sensor in "${ROBOT_SENSORS[@]}"; do
    CONFIG_FILE=""
    for ext in yaml json toml; do
        CANDIDATE="$RECIPES_ROOT/$RECIPE_NAME/${sensor}_config.$ext"
		# Check if the file exists
		if docker exec "$CONTAINER_NAME" bash -c "[[ -f '$CANDIDATE' ]]"; then
			CONFIG_FILE="$CANDIDATE"
			break
		fi
    done

    if [[ -n "$CONFIG_FILE" ]]; then
        run_with_spinner "Launching sensor: $sensor with custom config from $CONFIG_FILE..." \
            "docker exec -d $CONTAINER_NAME bash -c 'source ros_entrypoint.sh && ros2 launch $EMOS_ROOT/robot/launch/bringup_${sensor}.py config_file:=$CONFIG_FILE'" || exit 1
    else
        run_with_spinner "Launching sensor: $sensor with default config..." \
            "docker exec -d $CONTAINER_NAME bash -c 'source ros_entrypoint.sh && ros2 launch $EMOS_ROOT/robot/launch/bringup_${sensor}.py'" || exit 1
    fi
done


# --- Node Verification ---
print_header "VERIFYING ROS2 NODES"
sleep 5 # Give nodes a moment to initialize

ROBOT_JSON_FILE="$HOME/$EMOS_ROOT/robot/manifest.json"
mapfile -t EXPECTED_NODES < <(jq -r '.base[]' "$ROBOT_JSON_FILE")

for sensor in "${ROBOT_SENSORS[@]}"; do
  node=$(jq -r --arg s "$sensor" 'select(has($s))[$s] // empty' "$ROBOT_JSON_FILE")
  if [[ -n "$node" ]]; then
    EXPECTED_NODES+=("$node")
  fi
done

log "Verifying required ROS2 nodes are active..."
ALL_PRESENT=true
for NODE in "${EXPECTED_NODES[@]}"; do
    NODE_CLEAN=$(echo "$NODE" | xargs)

    # Use gum spin for the verification loop
    if ! gum spin --spinner dot --title "Checking for node '$NODE_CLEAN'..." -- \
        bash -c "for i in {1..10}; do docker exec $CONTAINER_NAME bash -c 'source ros_entrypoint.sh && ros2 node list' | grep -Fq \"$NODE_CLEAN\" && exit 0; sleep 1; done; exit 1"; then
        error "Node '$NODE_CLEAN' did not appear within 10 seconds!"
        ALL_PRESENT=false
    else
        success "Node '$NODE_CLEAN' is active."
    fi
done

if [[ "$ALL_PRESENT" == "false" ]]; then
  error "Required nodes are missing. Stopping container."
  docker stop $CONTAINER_NAME >/dev/null 2>&1
  exit 1
fi

# --- Final Configuration ---
print_header "FINAL CONFIGURATION"

if [[ "$ACTIVATE_AUTONOMOUS_MODE" == "true" ]]; then
  run_with_spinner "Activating autonomous mode..." \
    "docker exec -d $CONTAINER_NAME bash -c 'source ros_entrypoint.sh && ./$EMOS_ROOT/robot/scripts/activate_autonomous_mode.sh'" || exit 1
  warn "ATTENTION: Autonomous Mode is now ON."
else
  run_with_spinner "Deactivating autonomous mode..." \
    "docker exec -d $CONTAINER_NAME bash -c 'source ros_entrypoint.sh && ./$EMOS_ROOT/robot/scripts/deactivate_autonomous_mode.sh'" || exit 1
fi

# --- Recipe Execution ---
print_header "LAUNCHING RECIPE: $RECIPE_NAME"

mkdir -p "${LOG_DIR}"
log "All output will be saved to: ${LOG_FILE}"

if [[ "${RUN_WEB_CLIENT}" == "true" ]]; then
    run_with_spinner "Starting web client in background..." \
        "docker exec -d ${CONTAINER_NAME} bash -c 'source ros_entrypoint.sh && ros2 run automatika_embodied_agents tiny_web_client'" || exit 1
    log "Web client should be available at http://<ROBOT_IP>:8080"
fi

gum style --bold --padding "1 0" --foreground 2 "BEGIN RECIPE OUTPUT"

# Execute the main recipe, teeing output to the log file.
docker exec -it "${CONTAINER_NAME}" bash -c \
    "source ros_entrypoint.sh && python3 ${RECIPES_ROOT}/${RECIPE_NAME}/recipe.py" \
    | tee "${LOG_FILE}"

RECIPE_EXIT_CODE=${PIPESTATUS[0]}

gum style --bold --padding "1 0" --foreground 2 "END RECIPE OUTPUT"

if [ $RECIPE_EXIT_CODE -eq 0 ]; then
    success "Recipe '$RECIPE_NAME' finished successfully."
else
    error "Recipe '$RECIPE_NAME' exited with an error (code: $RECIPE_EXIT_CODE)."
fi

# Restart emos container for cleanup
run_with_spinner "EMOS container cleanup..." "docker stop $CONTAINER_NAME >/dev/null 2>&1" || true

