# emos-cli

<div align="center">
  <pre>
███████╗███╗   ███╗ ██████╗ ███████╗
██╔════╝████╗ ████║██╔═══██╗██╔════╝
█████╗  ██╔████╔██║██║   ██║███████╗
██╔══╝  ██║╚██╔╝██║██║   ██║╚════██║
███████╗██║ ╚═╝ ██║╚██████╔╝███████║
╚══════╝╚═╝     ╚═╝ ╚═════╝ ╚══════╝
  </pre>
  <p>
    <strong>A glamorous command-line tool for managing EMOS.</strong>
  </p>
  <p>
    Built with <a href="https://www.gnu.org/software/bash/">Bash</a> and <a href="https://github.com/charmbracelet/gum">gum</a> for a beautiful, interactive experience.
  </p>
</div>

`emos-cli` is the command-line interface for installing, updating, and managing the EMOS (EmbodiedOS, a robot's automation layer). It handles everything from license validation to container management, all behind a simple and elegant interface.

## 🚀 Quick Start: One-Line Installation

To install `emos-cli` on your system, simply run the following command in your terminal.

```bash
curl -sSL https://raw.githubusercontent.com/automatika-robotics/emos-cli/main/install.sh | sudo bash
````


## 📋 Prerequisites

Before installing, please ensure you have the following dependencies installed on your system:

1.  **Docker**: The `emos` tool is a Docker container manager. The CLI will not work without it.

      * ➡️ **[Install Docker Engine (Official Guide)](https://docs.docker.com/engine/install/)**

2.  **`curl`**: Used for downloading the installer and communicating with the license API. It is installed by default on most systems.

3.  **`sudo` Access**: The installer script requires administrative privileges to install dependencies and place the `emos` binary in `/usr/local/bin`.

The installer script will automatically attempt to install `gum` and `jq` using your system's package manager.

## 🌟 Features

  * **✨ Glamorous Interface**: Uses `gum` to provide spinners, styled text, and interactive prompts.
  * **🚀 One-Line Install**: Get up and running in seconds with a secure install script.
  * **🔑 License Management**: Easily install and validate your EmbodiedOS license key.
  * **🐳 Automated Container Management**: Handles pulling, running, and updating the EMOS container.
  * **🔄 Self-Updating CLI**: The `emos update` command can update both the CLI tool itself and the EMOS container.

## 💻 Installing EMOS with emos-cli

The `emos` CLI is simple and intuitive.

```bash
emos install YOUR-LICENSE-KEY
```

This is the first command you should run. It validates your license key, pulls the EMOS container image for your robot, starts it, and creates a systemd service for auto-restarts. License keys are bundled with supported robots.

After installation, checkout help for more functionality:

```bash
emos --help
```

## Copyright

The code in this distribution is Copyright (c) 2025 [Automatika Robotics](https://automatikarobotics.com/) unless explicitly indicated otherwise.

_emos-cli_ is made available under the MIT license. Details can be found in the [LICENSE](LICENSE) file.

