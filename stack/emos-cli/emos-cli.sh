#!/bin/bash

# ==============================================================================
# emos - EmbodiedOS Management CLI v0.3.5
# ==============================================================================

# --- Configuration ---
EMOS_VERSION="0.3.5"
CONFIG_DIR="$HOME/.config/emos"
RECIPES_DIR="$HOME/emos/recipes"
LICENSE_FILE="$CONFIG_DIR/license.key"
CONTAINER_NAME="emos"
SERVICE_NAME="emos.service"
GITHUB_ORG="automatika-robotics"
INSTALLER_URL="https://raw.githubusercontent.com/$GITHUB_ORG/emos-cli/main/install.sh"

# --- Support API Endpoints ---
API_BASE_URL="https://support-api.automatikarobotics.com/api"
CREDENTIALS_ENDPOINT="$API_BASE_URL/registrations/credentials"
RECIPES_LIST_ENDPOINT="$API_BASE_URL/recipes"
RECIPE_PULL_ENDPOINT="$API_BASE_URL/recipes/%s" # %s is a placeholder for the filename

# --- Globals & Helpers ---
# Get the directory where this script is located, even if it's a symlink
SCRIPT_SOURCE="${BASH_SOURCE[0]}"
while [ -h "$SCRIPT_SOURCE" ]; do # resolve $SCRIPT_SOURCE until the file is no longer a symlink
  SCRIPT_DIR="$( cd -P "$( dirname "$SCRIPT_SOURCE" )" >/dev/null 2>&1 && pwd )"
  SCRIPT_SOURCE="$(readlink "$SCRIPT_SOURCE")"
  [[ $SCRIPT_SOURCE != /* ]] && SCRIPT_SOURCE="$SCRIPT_DIR/$SCRIPT_SOURCE" # if $SCRIPT_SOURCE was a relative symlink
done
SCRIPT_DIR="$( cd -P "$( dirname "$SCRIPT_SOURCE" )" >/dev/null 2>&1 && pwd )"

# Source the library from the same directory
source "$SCRIPT_DIR/emos-lib.sh"

# Define paths to helper scripts
RECIPE_RUNNER="$SCRIPT_DIR/recipe_run.sh"
MAPPING_RUNNER="$SCRIPT_DIR/mapping_run.sh"

# --- Styling and UI Functions ---
display_art() {
    ART=$(cat << "EOF"
███████╗███╗   ███╗ ██████╗ ███████╗
██╔════╝████╗ ████║██╔═══██╗██╔════╝
█████╗  ██╔████╔██║██║   ██║███████╗
██╔══╝  ██║╚██╔╝██║██║   ██║╚════██║
███████╗██║ ╚═╝ ██║╚██████╔╝███████║
╚══════╝╚═╝     ╚═╝ ╚═════╝ ╚══════╝
EOF
)
    gum style --foreground $THEME_RED --padding "1 2" "$ART"
    gum style --padding "0 2" --foreground $THEME_BLUE --bold "EmbodiedOS Management CLI v${EMOS_VERSION}"
    echo
}

# --- Core Logic Functions ---
show_help() {
    gum style --bold --foreground $THEME_BLUE "EmbodiedOS Management CLI"
    echo "Usage: emos [command]"
    echo

    # Format the options to look like a table for the interactive menu
    gum style --bold --foreground $THEME_BLUE "? Select a command to generate a template:"
    local choice
    choice=$(gum choose --height 10 --cursor-prefix "➜ " --header " " \
        --item.foreground $THEME_NEUTRAL \
        --cursor.foreground $THEME_RED \
        "install   - Install and start EMOS using a license key." \
        "update    - Update the CLI and/or the EMOS container." \
        "ls        - List available automation recipes." \
        "run       - Execute a specific automation recipe." \
        "recipes   - List available recipes for download." \
        "pull      - Download and install a specific recipe." \
		"map       - Manage mapping functions (Run emos map to see subcommands)." \
        "status    - Display info and container status." \
        "version   - Show the current version of the CLI tool." \
        "exit      - Exit this menu.")

    # Check if the user made a choice (didn't press ESC)
    if [ -n "$choice" ]; then
        # Extract the command keyword from the choice (the first word)
        local command_keyword
        command_keyword=$(echo "$choice" | awk '{print $1}')

        case "$command_keyword" in
            "install")
                gum style --faint "Voilà ! Copy, add license key and press Enter."
                printf "emos install <license-key>\n"
                ;;
            "update")
                gum style --faint "Voilà ! Copy and press Enter."
                printf "emos update\n"
                ;;
            "ls")
                gum style --faint "Voilà ! Copy and press Enter."
                printf "emos ls\n"
                ;;
            "run")
                gum style --faint "Voilà ! Copy, add recipe name and optional arguments."
                printf "emos run <recipe_name>\n"
                ;;
            "recipes")
                gum style --faint "Voilà ! Copy and press Enter."
                printf "emos recipes\n"
                ;;
            "pull")
                gum style --faint "Voilà ! Copy, add recipe name and press Enter."
                printf "emos pull <recipe_name>\n"
                ;;
			"map")
                gum style --faint "Voilà ! Copy and press Enter."
                printf "emos map <command_name>\n"
                ;;
            "status")
                gum style --faint "Voilà ! Copy and press Enter."
                printf "emos status\n"
                ;;
            "version")
                gum style --faint "Voilà ! Copy and press Enter."
                printf "emos --version\n"
                ;;
            "exit")
                # Just exit cleanly with no output
                exit 0
                ;;
        esac
    fi
}

# Lists available automation recipes found in the recipes directory.
do_list_recipes() {
    # Check if the recipes directory exists and is not empty
    if [ ! -d "$RECIPES_DIR" ] || ! ls -d "$RECIPES_DIR"/*/ >/dev/null 2>&1; then
        gum style --foreground 3 "No recipes found in '$RECIPES_DIR'."
        return
    fi

    # Prepare the header for the table
    local table_data="NAME,DESCRIPTION"

    # Loop through each recipe's subdirectory
    for recipe_path in "$RECIPES_DIR"/*/; do
        local short_name
        short_name=$(basename "$recipe_path")

        local manifest_file="${recipe_path}manifest.json"
        local long_name="-" # Default value if manifest or name is missing

        if [ -f "$manifest_file" ]; then
            # Use jq to extract the 'name' field, ensuring it's not null or empty
            local name_from_json
            name_from_json=$(jq -r '.name' "$manifest_file")
            if [ -n "$name_from_json" ] && [ "$name_from_json" != "null" ]; then
                long_name="$name_from_json"
            fi
        fi

        # Append the new row to our CSV data, quoting the full name
        table_data+="\n$short_name,\"$long_name\""
    done

    gum style --bold --foreground "$THEME_BLUE" "Recipes available on the robot"
    # Pipe the CSV data into gum table for a pretty output
    echo -e "$table_data" | gum table --selected.foreground "$THEME_RED"

}

# Runs a specific automation recipe.
# Usage:do_run_recipe <recipe_name>
do_run_recipe() {
    local recipe_name="$1"

    # Check if a recipe name was provided
    if [ -z "$recipe_name" ]; then
        gum style --foreground 1 "✖ Error: A recipe name is required."
        echo "Usage: emos run <recipe_name>"
        gum style --faint "Run 'emos ls' to see a list of available recipes."
        exit 1
    fi

    # Shift the recipe_name off the argument list. The rest ($@) are extra args.
    shift

    # Check if the recipe_run.sh script exists in the correct location
    if [ ! -x "$RECIPE_RUNNER" ]; then
        gum style --foreground 1 "✖ Error: The 'recipe_run.sh' script is not found at $RECIPE_RUNNER."
        gum style --faint "Please ensure the EmbodiedOS execution environment is correctly installed."
        exit 1
    fi

    # Check if the recipe directory actually exists
    local recipe_path="$RECIPES_DIR/$recipe_name"
    if [ ! -d "$recipe_path" ]; then
        gum style --foreground 1 "✖ Error: Recipe '$recipe_name' not found in '$RECIPES_DIR'."
        gum style --faint "Please check the spelling. Run 'emos ls' to see available recipes."
        exit 1
    fi

    # If all checks pass, execute the recipe
    display_art
    gum style --bold --foreground "$THEME_RED" --padding "0 1" "🚀 Launching recipe: $recipe_name"
    echo

    # Execute the run script directly, allowing the user to see its output and interact.
    # The "$@" ensures extra arguments are passed along correctly.
    "$RECIPE_RUNNER" --recipe_name="$recipe_name" "$@"

    local exit_code=$?
    echo
    if [ $exit_code -eq 0 ]; then
        gum style --foreground 2 "✔ Recipe '$recipe_name' finished successfully."
    else
        gum style --foreground 1 "✖ Recipe '$recipe_name' exited with an error (code: $exit_code)."
    fi
}

# Lists available recipes from the remote API.
do_list_remote_recipes() {
    local TMP_FILE
    TMP_FILE=$(mktemp)

    # Fetch the list of recipes from the API
    if ! gum spin --spinner dot --title "Fetching available recipes from server..." -- \
        curl -sSLf "$RECIPES_LIST_ENDPOINT" -o "$TMP_FILE"; then
        rm "$TMP_FILE"
        gum style --foreground 1 "✖ Error: Could not connect to the recipes API."
        exit 1
    fi

    local API_RESPONSE
    API_RESPONSE=$(cat "$TMP_FILE")
    rm "$TMP_FILE"

    if [ -z "$API_RESPONSE" ] || ! echo "$API_RESPONSE" | jq empty; then
        gum style --foreground 1 "✖ Error: Recipes API returned an invalid or empty response."
        exit 1
    fi

    # Use jq to parse the JSON array and format it as CSV for gum table
    # It extracts the filename, and gets the full name.
    local table_data="RECIPE NAME,DESCRIPTION"
    table_data+=$(echo "$API_RESPONSE" | jq -r '.[] | "\n\(.filename),\"\(.name)\""')

    gum style --bold --foreground "$THEME_BLUE" "Available Recipes for Download"
    echo -e "$table_data" | gum table --selected.foreground "$THEME_RED"

    gum style --faint "Use 'emos pull <recipe_name>' to install a recipe."
}

# Downloads and unzips a specific recipe.
# Usage: do_pull_recipe <recipe_short_name>
do_pull_recipe() {
    local recipe_name="$1"

    if [ -z "$recipe_name" ]; then
        gum style --foreground 1 "✖ Error: A recipe name is required."
        echo "Usage: emos pull <recipe_name>"
        gum style --faint "Run 'emos recipes' to see a list of available recipes."
        exit 1
    fi

    # Define and create the final destination directory for this specific recipe
    local recipe_dest_dir="$RECIPES_DIR/$recipe_name"

    # Check for existing recipe and ask for confirmation before proceeding
    if [ -d "$recipe_dest_dir" ]; then
        gum style --foreground 3 --padding "0 1" "[!] Warning: A recipe named '$recipe_name' already exists."
        if ! gum confirm --prompt.foreground $THEME_BLUE --selected.background $THEME_RED "Continuing will overwrite its contents. Are you sure?"; then
            gum style --foreground 1 "✖ Pull operation aborted by user."
            exit 1
        fi
        echo # newline for better spacing
    fi

    # Reconstruct the zip filename and the download URL
    local download_url
    download_url=$(printf "$RECIPE_PULL_ENDPOINT" "$recipe_name")
    local temp_zip_path="/tmp/${recipe_name}.zip"

    # Ensure the destination directory exists
    mkdir -p "$recipe_dest_dir"

    gum style --bold --foreground "$THEME_BLUE" "Installing recipe: $recipe_name"

    # Download the recipe zip file
    run_with_spinner "Downloading from $download_url..." \
        "curl -sSLf \"$download_url\" -o \"$temp_zip_path\"" || exit 1

    # Unzip the file into the recipe-specific directory
    # The -o flag overwrites existing files without prompting
    run_with_spinner "Unzipping recipe to $recipe_dest_dir..." \
        "unzip -o \"$temp_zip_path\" -d \"$recipe_dest_dir\"" || exit 1

    # Clean up the downloaded zip file
    rm "$temp_zip_path"

    gum style --border double --padding "1 5" --border-foreground 2 "✔ Recipe '$recipe_name' installed successfully!"
}

show_status() {
    display_art
    echo "EMOS is a self-contained automation layer for your robot."
    echo "This tool helps you manage its lifecycle on this machine."
    echo "Developed with ❤️  by Automatika Robotics."
    echo
    gum style --bold "Container Status:"
    STATUS=$(docker inspect -f '{{.State.Status}}' "$CONTAINER_NAME" 2>/dev/null)
    if [ "$STATUS" == "running" ]; then
        STATUS_MSG=$(gum style --foreground 2 "● Running")
    elif [ "$STATUS" == "exited" ]; then
        STATUS_MSG=$(gum style --foreground 3 "● Exited")
    else
        STATUS_MSG=$(gum style --foreground 1 "● Not Found / Not Running")
    fi
    echo -e "  - Container '$CONTAINER_NAME': $STATUS_MSG"
    if [ -f "$LICENSE_FILE" ]; then
        echo -e "  - License Key: $(gum style --foreground 2 "Found")"
    else
        echo -e "  - License Key: $(gum style --foreground 1 "Not Found")"
    fi
}

# Validates a license key via the API and returns the JSON response.
# Exits with an error if validation fails.
# Usage: local API_RESPONSE=$(api_validate_license "<key>")
api_validate_license() {
    local license_key="$1"
    local TMP_FILE
    TMP_FILE=$(mktemp)

    if ! gum spin --spinner dot --title "Validating license key..." -- \
        curl -s -X POST \
            -H "Content-Type: application/json" \
            -d "{\"license_key\": \"$license_key\"}" \
            "$CREDENTIALS_ENDPOINT" -o "$TMP_FILE"; then
        rm "$TMP_FILE"
        gum style --foreground 1 "✖ Error: Could not connect to the license API. Check your network connection."
        exit 1
    fi

    local api_response
    api_response=$(cat "$TMP_FILE")
    rm "$TMP_FILE"

    if [ -z "$api_response" ] || ! echo "$api_response" | jq empty; then
        gum style --foreground 1 "✖ Error: API returned an invalid or empty response."
        exit 1
    fi
    if echo "$api_response" | jq -e '.error' > /dev/null; then
        local ERROR_MSG
        ERROR_MSG=$(echo "$api_response" | jq -r '.error')
        gum style --foreground 1 "✖ API Error: $ERROR_MSG"
        exit 1
    fi

    echo "$api_response"
}

# Parses the API JSON response, validates the contents, and exports them as global variables.
# Exits with an error if data is incomplete.
# Usage: parse_and_export_docker_vars "<json_string>"
parse_and_export_docker_vars() {
    local api_response="$1"

    export DOCKER_REGISTRY=$(echo "$api_response" | jq -r '.container_registry')
    export DOCKER_IMAGE=$(echo "$api_response" | jq -r '.image_name')
    export DOCKER_USER=$(echo "$api_response" | jq -r '.username')
    export DOCKER_TOKEN=$(echo "$api_response" | jq -r '.password')
    export DEPLOYMENT_REPO=$(echo "$api_response" | jq -r '.deployment_repository_name')


    if [ -z "$DOCKER_REGISTRY" ] || [ "$DOCKER_REGISTRY" == "null" ] || \
       [ -z "$DOCKER_USER" ] || [ "$DOCKER_USER" == "null" ] || \
       [ -z "$DOCKER_TOKEN" ] || [ "$DOCKER_TOKEN" == "null" ] || \
       [ -z "$DOCKER_IMAGE" ] || [ "$DOCKER_IMAGE" == "null" ] || \
       [ -z "$DEPLOYMENT_REPO" ] || [ "$DEPLOYMENT_REPO" == "null" ]; then

        gum style --foreground 1 "✖ Error: The license API returned incomplete or null data."
        gum style --faint "This can happen with an invalid license key. Please check the response below."
        echo "$api_response" | jq .
        exit 1
    fi
}

# Force-removes the main container if it exists.
remove_existing_container() {
    # Check if the container exists before trying to remove it.
    if docker inspect "$CONTAINER_NAME" &> /dev/null; then
        gum style --bold --foreground "$THEME_BLUE" "Removing existing EmbodiedOS container..."
        # Use a single, atomic force-remove command.
        run_with_spinner "Forcibly removing container '$CONTAINER_NAME'..." \
            "docker rm -f \"$CONTAINER_NAME\"" || exit 1
    fi
}

# Performs the Docker login, pull, and run sequence.
# Uses the globally exported Docker variables.
# Usage: deploy_container
deploy_container() {
    export FULL_IMAGE_NAME="${DOCKER_REGISTRY}/${DOCKER_IMAGE}"

    run_with_spinner "Logging into Docker registry..." \
        "echo \"$DOCKER_TOKEN\" | docker login \"$DOCKER_REGISTRY\" -u \"$DOCKER_USER\" --password-stdin" || exit 1

    gum style --bold --foreground "$THEME_BLUE" "Pulling EmbodiedOS container image..."
    gum style --faint "This may take several minutes depending on your network connection."
    docker pull "$FULL_IMAGE_NAME" || {
        gum style --foreground 1 "✖ Error: Failed to pull Docker image."
        exit 1
    }
    gum style --foreground 2 "✔ Success: Pulled latest image."

    run_with_spinner "Starting EmbodiedOS container..." \
        "docker run -d -it --restart always --privileged -v /dev/bus/usb:/dev/bus/usb -v $HOME/emos:/emos --name \"$CONTAINER_NAME\" --network host --runtime nvidia --gpus=all \"$FULL_IMAGE_NAME\"" || exit 1
}

# Clones the deployment repo, and copies the 'robot' directory into ~/emos.
# For update it removes the old 'robot' directory first.
deploy_robot_files() {
    gum style --bold --foreground "$THEME_BLUE" "Deploying robot configuration files..."

    local TMP_DIR
    TMP_DIR=$(mktemp -d)

    # Construct the authenticated Git URL
    local GIT_URL="https://""$DOCKER_USER"":""$DOCKER_TOKEN""@github.com/""$GITHUB_ORG""/""$DEPLOYMENT_REPO"".git"

    # Use --depth 1 for a faster, shallow clone
    run_with_spinner "Cloning deployment repository..." \
        "git clone --depth 1 \"$GIT_URL\" \"$TMP_DIR\"" || { rm -rf "$TMP_DIR"; exit 1; }

    # Check that the 'robot' directory exists in the cloned repo
    if [ ! -d "$TMP_DIR/robot" ]; then
        gum style --foreground 1 "✖ Error: Cloned repository does not contain a 'robot' directory."
        rm -rf "$TMP_DIR"
        exit 1
    fi

    gum style --faint "Updating local robot files at $HOME/emos/robot..."
    # Ensure a clean state by removing any old version
    rm -rf "$HOME/emos/robot"
    # Copy the new version into place
    cp -r "$TMP_DIR/robot" "$HOME/emos/"

    # Clean up the temporary clone
    rm -rf "$TMP_DIR"
    gum style --foreground 2 "✔ Success: Robot files deployed."
}


do_install() {
    local license_key="$1"
    if [ -z "$license_key" ]; then
        gum style --foreground 1 "✖ Error: A license key is required."
        echo "Usage: emos install <your_license_key>"
        exit 1
    fi

    # Check for existing installation and confirm overwrite
    if docker inspect "$CONTAINER_NAME" &> /dev/null; then
        gum style --foreground 3 --padding "1 2" --border double --border-foreground 3 \
            "[!] An existing EmbodiedOS container was found."
        if ! gum confirm --prompt.foreground $THEME_BLUE --selected.background $THEME_RED "This will REMOVE the existing container and perform a fresh installation. Are you sure?"; then
            gum style --foreground 1 "✖ Installation aborted by user."
            exit 1
        fi
    fi

    display_art
    echo "🚀 Starting EmbodiedOS installation..."
    mkdir -p "$CONFIG_DIR"

    local API_RESPONSE
    API_RESPONSE=$(api_validate_license "$license_key")
    gum style --foreground 2 "✔ License key validated successfully."

    # Remove old container now that license is confirmed valid
    remove_existing_container

    parse_and_export_docker_vars "$API_RESPONSE"
    echo "$license_key" > "$LICENSE_FILE" # Save the license key

    # Create emos content dir
    gum style --faint "Ensuring local directory exists at $HOME/emos..."
    mkdir -p "$HOME/emos"

    deploy_robot_files

    deploy_container

    # --- install: systemd service ---
    gum style --bold --foreground "$THEME_BLUE" "Creating systemd service..."
    SERVICE_FILE_CONTENT=$(cat <<EOF
[Unit]
Description=EmbodiedOS Container
After=docker.service
Requires=docker.service
[Service]
Restart=always
ExecStart=/usr/bin/docker start -a $CONTAINER_NAME
ExecStop=/usr/bin/docker stop -t 2 $CONTAINER_NAME
[Install]
WantedBy=multi-user.target
EOF
)
    if gum confirm --prompt.foreground $THEME_BLUE --selected.background $THEME_RED "Create/overwrite systemd service file for auto-restart?"; then
        echo "$SERVICE_FILE_CONTENT" | sudo tee "/etc/systemd/system/${SERVICE_NAME}" > /dev/null
        run_with_spinner "Reloading systemd daemon..." "sudo systemctl daemon-reload"
        run_with_spinner "Enabling emos service..." \
            "sudo systemctl enable ${SERVICE_NAME}" || exit 1
        run_with_spinner "Activating emos service..." \
            "sudo systemctl start ${SERVICE_NAME}" || exit 1
    else
        gum style --foreground 3 "Skipping systemd service creation."
    fi

    gum style --border double --padding "1 5" --border-foreground 2 "🎉 EmbodiedOS installed successfully! 🎉"
}

do_update() {
    display_art

    gum spin --spinner dot --title "Checking for emos CLI updates..." -- sleep 1
    echo "The update process requires administrative privileges to check/update the CLI."
    set -o pipefail
    curl -sSL "$INSTALLER_URL" | sudo bash -s -- update
    CLI_UPDATE_STATUS=$?
    set +o pipefail
    if [ "$CLI_UPDATE_STATUS" -eq 10 ]; then
        gum style --foreground "$THEME_BLUE" "💡 The emos CLI has been updated to the latest version."
        gum style --bold "Please run 'emos update' again to update your EmbodiedOS container."
        exit 0
    elif [ "$CLI_UPDATE_STATUS" -ne 0 ]; then
        gum style --foreground 1 "✖ Error: The CLI update check failed. Cannot proceed."
        exit 1
    fi
    gum style --foreground 2 "✔ Your emos CLI is up to date."
    echo

    # Check for license file before proceeding
    if [ ! -f "$LICENSE_FILE" ]; then
        gum style --foreground 1 "✖ Error: No existing installation found."
        echo "Please run 'emos install <license_key>' first."
        exit 1
    fi
    echo "🚀 Checking for EmbodiedOS container updates..."
    LICENSE_KEY=$(cat "$LICENSE_FILE")

    local API_RESPONSE
    API_RESPONSE=$(api_validate_license "$LICENSE_KEY")
    gum style --foreground 2 "✔ License verified."

    parse_and_export_docker_vars "$API_RESPONSE"

    remove_existing_container

    deploy_robot_files

    deploy_container

    gum style --border double --padding "1 5" --border-foreground 2 "✅ EmbodiedOS container updated successfully! ✅"
}

# --- Mapping --- #

# Displays the help menu for the 'map' command.
show_map_help() {
    gum style --bold --foreground $THEME_BLUE "EMOS Map Management Commands"
    echo "Usage: emos map <subcommand>"
    echo
    echo "Available subcommands:"
    gum style --padding "0 2" "  record   - Start recording a new map (Should be run on board the robot)."
    gum style --padding "0 2" "  install-editor - Download and install the map editor container."
	gum style --padding "0 2" "  edit <path>    - Process a ROS bag file to generate a PCD map."
    echo
    gum style --faint "Run commands with emos map <command_name>"
}

# Runs mapping data recording on the robot.
# Usage:do_map_record
do_map_record() {

    # Check if the mapping_run.sh script exists in the correct location
    if [ ! -x "$MAPPING_RUNNER" ]; then
        gum style --foreground 1 "✖ Error: The 'mapping_run.sh' script is not found at $MAPPING_RUNNER."
        gum style --faint "Please ensure the EmbodiedOS execution environment is correctly installed."
        exit 1
    fi

    # If all checks pass, execute the recipe
    display_art
    gum style --bold --foreground "$THEME_RED" --padding "0 1" "🚀 Launching mapping"
    echo

    # Execute the run script directly, allowing the user to see its output and interact.
    "$MAPPING_RUNNER"

    local exit_code=$?
    echo
    if [ $exit_code -eq 0 ]; then
        gum style --foreground 2 "✔ Mapping data recording finished successfully."
    else
        gum style --foreground 1 "✖ Mapping data recording exited with an error (code: $exit_code)."
    fi
}

# Downloads and installs the map editor container.
do_map_install_editor() {
    display_art
    gum style --bold --foreground "$THEME_BLUE" "🚀 Starting Map Editor Installation..."
    echo

    local license_key
    license_key=$(gum input --placeholder "Please enter your license key to proceed")

    if [ -z "$license_key" ]; then
        gum style --foreground 1 "✖ Error: A license key is required."
        exit 1
    fi

    local API_RESPONSE
    API_RESPONSE=$(api_validate_license "$license_key")
    gum style --foreground 2 "✔ License key validated successfully."

    parse_and_export_docker_vars "$API_RESPONSE"

    # Check for and remove existing mapping container
    local MAPPING_CONTAINER_NAME="emos-mapping"
    if docker inspect "$MAPPING_CONTAINER_NAME" &> /dev/null; then
        gum style --foreground 3 --padding "0 1" "[!] An existing '$MAPPING_CONTAINER_NAME' container was found."
        if ! gum confirm --prompt.foreground $THEME_BLUE --selected.background $THEME_RED "Overwrite existing map editor container?"; then
            gum style --foreground 1 "✖ Installation aborted by user."
            exit 1
        fi
        run_with_spinner "Removing existing '$MAPPING_CONTAINER_NAME' container..." \
            "docker rm -f \"$MAPPING_CONTAINER_NAME\"" || exit 1
    fi

    # Log into docker
    run_with_spinner "Logging into Docker registry..." \
        "echo \"$DOCKER_TOKEN\" | docker login \"$DOCKER_REGISTRY\" -u \"$DOCKER_USER\" --password-stdin" || exit 1

    # Prepare the mapping image name
    local BASE_IMAGE="${DOCKER_IMAGE%:*}" # Removes tag
    local MAPPING_IMAGE_NAME="${BASE_IMAGE}:mapping"
    export FULL_IMAGE_NAME="${DOCKER_REGISTRY}/${MAPPING_IMAGE_NAME}"

    # Pull the image
    gum style --bold --foreground "$THEME_BLUE" "Pulling Map Editor container image: $FULL_IMAGE_NAME"
    gum style --faint "This may take several minutes depending on your network connection."
    docker pull "$FULL_IMAGE_NAME" || {
        gum style --foreground 1 "✖ Error: Failed to pull Docker image."
        exit 1
    }
    gum style --foreground 2 "✔ Success: Pulled latest image."

    # Run the container interactively to "install" it (create it with the right settings)
    gum style --bold --foreground "$THEME_BLUE" "Creating '$MAPPING_CONTAINER_NAME' container..."

    local MAPPING_DOCKER_RUN_CMD="docker run -d --name=\"$MAPPING_CONTAINER_NAME\" -it --device=/dev/dri --group-add video --volume=/tmp/.X11-unix:/tmp/.X11-unix --env=\"DISPLAY=$DISPLAY\" \"$FULL_IMAGE_NAME\""

    eval "$MAPPING_DOCKER_RUN_CMD"
    local exit_code=$?
    echo

    run_with_spinner "Stopping and finalizing installation..." \
        "docker stop '$MAPPING_CONTAINER_NAME'" || exit 1

    if [ $exit_code -eq 0 ]; then
        gum style --border double --padding "1 5" --border-foreground 2 "✔ Map Editor installed successfully!"
        gum style --faint "You can start it later with 'emos map start-editor'"
    else
        gum style --foreground 1 "✖ Map Editor installation failed (code: $exit_code)."
    fi
}

# Processes a ROS bag file to generate a PCD map using the editor container.
# Usage: do_map_edit <path_to_bag_file.tar.gz>
do_map_edit() {
    local bag_file_path="$1"
    local MAPPING_CONTAINER_NAME="emos-mapping"

    # Argument and File Validation
    if [ -z "$bag_file_path" ]; then
        gum style --foreground 1 "✖ Error: Path to a ROS bag file (.tar.gz) is required."
        echo "Usage: emos map edit <path_to_bag_file.tar.gz>"
        exit 1
    fi
    if [[ "$bag_file_path" != *.tar.gz ]]; then
        gum style --foreground 1 "✖ Error: File must be a .tar.gz archive."
        exit 1
    fi
    if [ ! -f "$bag_file_path" ]; then
        gum style --foreground 1 "✖ Error: File not found at '$bag_file_path'."
        exit 1
    fi

    # Extract names from the path
    local bag_filename
    bag_filename=$(basename "$bag_file_path")
    local map_name
    map_name="${bag_filename%.tar.gz}" # Removes .tar.gz from filename

    # Check for and start the container
    if ! docker inspect "$MAPPING_CONTAINER_NAME" &> /dev/null; then
        gum style --foreground 1 "✖ Error: Mapping container '$MAPPING_CONTAINER_NAME' not found."
        gum style --faint "Please run 'emos map install-editor' first."
        exit 1
    fi

    # Stop the container if its started
    if [ "$(docker ps -q -f name=$MAPPING_CONTAINER_NAME)" ]; then
        run_with_spinner "Stopping existing EMOS container..." "docker stop $MAPPING_CONTAINER_NAME >/dev/null 2>&1" || true
    fi

    run_with_spinner "Starting map editor container..." \
        "docker start '$MAPPING_CONTAINER_NAME'" || exit 1

    display_art
    gum style --bold --foreground "$THEME_BLUE" "🚀 Processing Map File: $map_name"

    # Allow GUI access from the container
    gum style --faint "Temporarily allowing container GUI access..."
    xhost +local:docker &>/dev/null

    # Copy and extract the bag file inside the container
    run_with_spinner "Copying '$bag_filename' into container..." \
        "docker cp '$bag_file_path' '$MAPPING_CONTAINER_NAME:/tmp/'" || exit 1
    run_with_spinner "Extracting bag file inside container..." \
        "docker exec '$MAPPING_CONTAINER_NAME' tar -xzf '/tmp/$bag_filename' -C /tmp/" || exit 1

    # Run the editor process in the container (detached)
    gum style --bold --foreground "$THEME_BLUE" "Starting the editor..."
    docker exec -d "$MAPPING_CONTAINER_NAME" bash -c "
        source /ros_entrypoint.sh
        ros2 run glim_ros glim_roseditor --map_path /tmp/dump --save_path /tmp --map_name $map_name
        "

    # RUN Glim TF node (transforms from raw robot topics to glim topics with unique base_link frame)
    gum style --bold --foreground "$THEME_BLUE" "Starting Data TF..."
    docker exec -d "$MAPPING_CONTAINER_NAME" bash -c "
        source /ros_entrypoint.sh
        ros2 run glim_ros glim_transformer_node /lidar/raw /imu/raw /lidar /imu  base_link
        "

    # Start the ROS bag playback (detached)
    gum style --bold --foreground "$THEME_BLUE" "Starting ROS bag playback..."
    docker exec -d "$MAPPING_CONTAINER_NAME" bash -c "source /ros_entrypoint.sh && ros2 bag play /tmp/$map_name/*"

    # Wait for the editor process to finish
    gum style --bold --foreground "$THEME_BLUE" "Edit your map and close the editor window when done. I will wait for you to finish..."

    while docker top "$MAPPING_CONTAINER_NAME" | grep -q glim_roseditor; do
        sleep 5
    done

    gum style --foreground 2 "✔ Editor process has exited."

    # Once exited, check that the PCD file exists
    local output_pcd_path_container="/tmp/$map_name.pcd"
    if docker exec "$MAPPING_CONTAINER_NAME" test -f "$output_pcd_path_container"; then
        gum style --foreground 2 "✔ PCD file generated successfully inside the container."
    else
        gum style --foreground 1 "✖ Error: Editor exited but no PCD file found at $output_pcd_path_container."
        exit 1
    fi

    # Copy the output file from the container to the host
    local host_output_path="./${map_name}.pcd"
    run_with_spinner "Copying '$map_name.pcd' to host at '$host_output_path'..." \
        "docker cp '$MAPPING_CONTAINER_NAME:$output_pcd_path_container' '$host_output_path'" || exit 1

    # Cleanup
    gum style --faint "Cleaning up..."
    # Clean up files inside the container in the background
    docker exec "$MAPPING_CONTAINER_NAME" rm -rf "/tmp/$map_name" "/tmp/dump" "/tmp/$bag_filename" "$output_pcd_path_container"
    run_with_spinner "Stopping the map editor container..." \
        "docker stop '$MAPPING_CONTAINER_NAME'" || exit 1
    xhost -local:docker &>/dev/null

    gum style --border double --padding "1 5" --border-foreground 2 "🎉 Map editing complete! Your map is ready at '$host_output_path' 🎉"
}

# Handles 'map' subcommands
# Usage: handle_map_command <subcommand> [args...]
handle_map_command() {
    local subcommand="$1"
    shift # Remove subcommand from arg list

    case "$subcommand" in
        record)
            do_map_record
            ;;
        install-editor)
            do_map_install_editor
            ;;
        edit)
            do_map_edit "$@" # Pass the file path
            ;;
        "" | "help" | "--help")
            show_map_help
            ;;
        *)
            gum style --foreground 1 "✖ Error: Unknown map subcommand '$subcommand'"
            show_map_help
            exit 1
            ;;
    esac
}

# --- Main Execution ---
main() {
    check_dependencies
    case "$1" in
        install)
            do_install "$2"
            ;;
        update)
            do_update
            ;;
        ls)
            do_list_recipes
            ;;
        run)
            shift
            do_run_recipe "$@" # Pass all remaining arguments
            ;;
        recipes)
            do_list_remote_recipes
            ;;
        pull)
            do_pull_recipe "$2"
            ;;
        map)
            shift
            handle_map_command "$@" # Pass all subcommands to handler
            ;;
        status)
            show_status
            ;;
        --version)
            display_art
            echo "EMOS is a self-contained automation layer for your robot."
            echo "This tool helps you manage its lifecycle on this machine."
            echo "Developed with ❤️  by Automatika Robotics."
            ;;
        --help)
            show_help
            ;;
        "")
            show_help
            ;;
        *)
            gum style --foreground 1 "✖ Error: Unknown command '$1'"
            echo "Run 'emos --help' to see a list of available commands."
            exit 1
            ;;
    esac
}
main "$@"
