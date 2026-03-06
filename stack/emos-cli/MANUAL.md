# Emos CLI User Guide

## 1. Introduction

Welcome to the official user guide for the `emos` Command-Line Interface (CLI).

The `emos` CLI is a powerful, user-friendly tool designed to install, manage, update, and run recipes on EMOS (EmbodiedOS). It provides a simple yet "glamorous" interface to streamline all interactions with your robot's software stack, from initial setup to recipe execution.

This guide will walk you through every command, explaining its purpose, usage, and arguments in detail.

## 2. Installation

The `emos` CLI is installed using a simple, secure one-line command. Open your terminal and run the following:

```bash
curl -sSL [https://raw.githubusercontent.com/automatika-robotics/emos-cli/main/install.sh](https://raw.githubusercontent.com/automatika-robotics/emos-cli/main/install.sh) | sudo bash
```

This command downloads the official installer script, which then:

1.  Checks for dependencies like `gum`, `curl`, and `jq` and installs them if missing.
2.  Downloads the latest versions of the `emos` and `recipe_run.sh` executables.
3.  Places them in `/usr/local/bin`, making them available system-wide.

## 3. Core Concepts

- **Recipe:** A recipe is a self-contained automation script that defines a specific robot behavior (e.g., person following, path following with way point stops etc).
- **Recipe Short Name:** The concise, machine-friendly name of a recipe, used in commands like `emos pull` and `emos run`. It's typically derived from the recipe's filename (e.g., `recipe_one`).
- **Local Recipes:** Recipes that have been downloaded and are physically present on your machine in the `~/emos/recipes/` directory.
- **Remote Recipes:** Recipes that are available for download from the central server.

## 4. Command Reference

This section details every available command in the `emos` CLI.

---

### `emos install`

Installs the complete EmbodiedOS environment on your machine. This is the first command you should run.

- **Usage:** `emos install <license_key>`
- **Description:** This command orchestrates the entire setup process. It validates your license key with the API, which returns credentials and configuration details. It then clones the necessary robot deployment files, pulls the specified Docker container image, starts the container, and sets up a systemd service to ensure the container automatically restarts on boot.
- **Arguments:**
  - `<license_key>`: (Required) Your unique license key for EmbodiedOS, issues by Automatika Robotics.
- **Notes:**
  - If an existing `emos` container is found, you will be prompted to confirm before it is removed and replaced.

---

### `emos update`

Updates both the `emos` CLI tools and the core EmbodiedOS container/deployment to their latest versions.

- **Usage:** `emos update`
- **Description:** This command performs a comprehensive two-stage update.
  1.  **CLI Update:** It first checks if a newer version of the emos CLI itself is available and updates it if necessary.
  2.  **System Update:** It then re-validates your license, downloads the latest robot deployment files (overwriting the old ones), stops and removes the old container, pulls the latest Docker image, and starts a new container.
- **Notes:**
  - This is the recommended way to keep your core system up-to-date.
  - You must have already run `emos install` at least once.

---

### `emos status`

Displays the current status of the local EMOS container.

- **Usage:** `emos status`
- **Description:** This command provides a quick overview of your local installation. It indicates whether the EmbodiedOS Docker container is currently running, stopped, or not found.

---

### `emos ls`

Lists all recipes that are currently installed on your local machine.

- **Usage:** `emos ls`
- **Description:** This command scans the `~/emos/recipes/` directory and displays a table of all found recipes, showing their **Short Name** (directory name) and **Description** (from the `manifest.json` file).

---

### `emos recipes`

Fetches and lists all recipes that are available for download from the remote server.

- **Usage:** `emos recipes`
- **Description:** This command contacts the EmbodiedOS API and displays a table of all officially available recipes, showing their **Short Name** and **Description**. Use this command to discover new recipes you can install.

---

### `emos pull`

Downloads and installs (or updates) a specific recipe from the remote server.

- **Usage:** `emos pull <recipe_short_name>`
- **Description:** This command takes the short name of a recipe, downloads the corresponding `.zip` file, and automatically extracts it into your local `~/emos/recipes/` directory. **If the recipe already exists, this command will overwrite it with the latest version from the server, effectively updating it.**
- **Arguments:**
  - `<recipe_short_name>`: (Required) The short name of the recipe you wish to download.

---

### `emos run`

Executes a locally installed recipe.

- **Usage:** `emos run <recipe_short_name>`
- **Description:** This command initiates a recipe. It handles the complex process of starting the Docker container, launching hardware drivers and ROS2 nodes, and finally executing the recipe's main Python script. All output from the recipe is streamed directly to your terminal.
- **Arguments:**
  - `<recipe_short_name>`: (Required) The short name of a local recipe you want to run (as seen in `emos ls`).

---

### `emos --help`

Displays an interactive help menu.

- **Usage:** `emos --help` or `emos`
- **Description:** Shows a list of all available commands with their descriptions. This menu is interactive; you can use the arrow keys to select a command and press `Enter` to have a runnable template of that command printed to your terminal.

---

### `emos --version`

Displays the current version of the `emos` CLI tool.

- **Usage:** `emos --version`

## 6\. Typical Workflow

Here is a common sequence of commands for a new user:

1.  **Install the system:**
    ```bash
    emos install YOUR-LICENSE-KEY-HERE
    ```
2.  **Discover available recipes:**
    ```bash
    emos recipes
    ```
3.  **Download a recipe you're interested in:**
    ```bash
    emos pull vision_follower
    ```
4.  **Confirm the recipe is installed locally:**
    ```bash
    emos ls
    ```
5.  **Run the recipe:**
    ```bash
    emos run vision_follower
    ```
6.  **Update an individual recipe later:**
    ```bash
    emos pull vision_follower
    ```
7.  **Update the core system periodically:**
    ```bash
    emos update
    ```

<!-- end list -->
