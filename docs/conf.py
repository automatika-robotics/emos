# Configuration file for the Sphinx documentation builder.
import os
import sys
import shutil
from datetime import date
from pathlib import Path

# Prevent runtime dependency checks in submodule packages
os.environ["KOMPASS_DOCS_BUILD"] = "1"
os.environ["AGENTS_DOCS_BUILD"] = "1"

project = "EMOS"
copyright = f"{date.today().year}, Automatika Robotics"
author = "Automatika Robotics"
release = "1.0"

extensions = [
    "sphinx.ext.viewcode",
    "sphinx.ext.doctest",
    "sphinx_copybutton",
    "autodoc2",
    "myst_parser",
    "sphinx_sitemap",
    "sphinx_design",
]

# -- autodoc2: all three packages in one build
autodoc2_packages = [
    {
        "module": "agents",
        "path": "../stack/embodied-agents/agents",
        "exclude_dirs": ["__pycache__", "utils"],
        "exclude_files": [
            "callbacks.py",
            "publisher.py",
            "component_base.py",
            "model_component.py",
            "model_base.py",
            "db_base.py",
            "executable.py",
        ],
    },
    {
        "module": "kompass",
        "path": "../stack/kompass/kompass/kompass",
        "exclude_files": [
            "utils.py",
            "components/utils.py",
        ],
    },
    {
        "module": "ros_sugar",
        "path": "../stack/sugarcoat/ros_sugar",
        "exclude_files": [
            "utils.py",
            "io/utils.py",
        ],
    },
]

autodoc2_docstrings = "all"
autodoc2_class_docstring = "both"
autodoc2_render_plugin = "myst"
autodoc2_hidden_objects = ["private", "dunder", "undoc"]
autodoc2_module_all_regexes = [
    r"agents.config",
    r"agents.models",
    r"agents.vectordbs",
    r"agents.ros",
    r"agents.clients\.[^\.]+",
    r"components\*",
    r"core\*",
]

templates_path = ["_templates"]
exclude_patterns = ["_build", "Thumbs.db", ".DS_Store", "README*"]

# Suppress ambiguous cross-reference warnings for types (Topic, Path, QoSConfig)
# that are re-exported identically in agents, kompass, and ros_sugar packages.
suppress_warnings = ["ref.python"]

# -- MyST configuration
myst_enable_extensions = [
    "amsmath",
    "attrs_inline",
    "colon_fence",
    "deflist",
    "dollarmath",
    "fieldlist",
    "html_admonition",
    "html_image",
    "linkify",
    "replacements",
    "smartquotes",
    "strikethrough",
    "substitution",
    "tasklist",
]
myst_heading_anchors = 7

# -- HTML output
html_baseurl = "https://emos.automatikarobotics.com/"
language = "en"
html_theme = "shibuya"
html_static_path = ["_static"]
html_css_files = ["custom.css"]
html_favicon = "_static/favicon.png"
sitemap_url_scheme = "{link}"

html_theme_options = {
    "light_logo": "_static/automatika-logo.png",
    "dark_logo": "_static/automatika-logo.png",
    "accent_color": "indigo",
    "twitter_url": "https://x.com/__automatika__",
    "github_url": "https://github.com/automatika-robotics/emos",
    "discord_url": "https://discord.gg/B9ZU6qjzND",
    "globaltoc_expand_depth": 1,
    "open_in_chatgpt": True,
    "open_in_claude": True,
    "nav_links": [
        {"title": "Automatika Robotics", "url": "https://automatikarobotics.com/"},
    ],
}


# --- LLMS.TXT CONFIGURATION ---
# Curated list of docs to include in llms.txt, ordered as a curriculum
# for AI agents learning to write EMOS recipes.
LLMS_TXT_SELECTION = [
    # Overview & Setup
    "overview.md",
    "getting-started/installation.md",
    "getting-started/quickstart.md",
    "getting-started/cli.md",
    # Core Concepts -- understand the architecture before writing code
    "concepts/architecture.md",
    "concepts/components.md",
    "concepts/topics.md",
    "concepts/events-and-actions.md",
    "concepts/status-and-fallbacks.md",
    "concepts/launcher.md",
    # Intelligence Layer (EmbodiedAgents) -- components, clients, models
    "intelligence/overview.md",
    "intelligence/ai-components.md",
    "intelligence/clients.md",
    "intelligence/models.md",
    # Navigation Layer (Kompass) -- robot config, planning, control
    "navigation/overview.md",
    "navigation/robot-config.md",
    "navigation/planning.md",
    "navigation/control.md",
    "navigation/drive-manager.md",
    "navigation/mapping.md",
    "navigation/motion-server.md",
    # Foundation Recipes -- from simple to complete agent
    "recipes/foundation/conversational-agent.md",
    "recipes/foundation/prompt-engineering.md",
    "recipes/foundation/semantic-map.md",
    "recipes/foundation/goto-navigation.md",
    "recipes/foundation/tool-calling.md",
    "recipes/foundation/semantic-routing.md",
    "recipes/foundation/complete-agent.md",
    # Planning & Manipulation Recipes
    "recipes/planning-and-manipulation/planning-models.md",
    "recipes/planning-and-manipulation/vla-manipulation.md",
    "recipes/planning-and-manipulation/event-driven-vla.md",
    # Navigation Recipes
    "recipes/navigation/simulation-quickstarts.md",
    "recipes/navigation/point-navigation.md",
    "recipes/navigation/vision-tracking.md",
    # Events & Resilience Recipes
    "recipes/events-and-resilience/multiprocessing.md",
    "recipes/events-and-resilience/fallback-recipes.md",
    "recipes/events-and-resilience/event-driven-cognition.md",
    # Advanced -- configuration, extending, algorithms
    "advanced/configuration.md",
    "advanced/extending.md",
    "advanced/types.md",
    "advanced/algorithms.md",
]


# --- Post-build hooks ---

def format_for_llm(filename: str, content: str) -> str:
    """Helper to wrap content in a readable format for LLMs."""
    lines = content.split("\n")
    cleaned_lines = [line for line in lines if "<img src=" not in line]
    cleaned_content = "\n".join(cleaned_lines).strip()
    return f"## File: {filename}\n```markdown\n{cleaned_content}\n```\n\n"


def generate_llms_txt(app, exception):
    """Generates llms.txt combining curated docs into an AI-friendly context file."""
    if exception is not None:
        return

    print("[llms.txt] Starting generation...")

    src_dir = Path(app.srcdir)
    out_dir = Path(app.outdir)
    full_text = []

    preamble = (
        "# EMOS Documentation -- Context for AI Agents\n\n"
        "You are an expert EMOS recipe developer. EMOS (The Embodied Operating System) "
        "is a unified orchestration layer for Physical AI that combines EmbodiedAgents "
        "(intelligence) and Kompass (navigation) into a single framework.\n\n"
        "## How to Write an EMOS Recipe\n\n"
        "An EMOS Recipe is a pure Python script that defines a robot behavior. "
        "When writing recipes, follow these principles:\n\n"
        "1. **Define Topics** -- Declare ROS2 topics as `Topic(name=..., msg_type=...)` "
        "for inter-component communication. Match `msg_type` to your data "
        "(String, Image, Audio, Detections, etc.).\n"
        "2. **Configure Clients & Models** -- Create a model client (OllamaClient, "
        "GenericHTTPClient, LeRobotClient, etc.) with a model wrapper. Clients are "
        "interchangeable -- swap inference backends without changing component logic.\n"
        "3. **Build Components** -- Instantiate components (LLM, VLM, VLA, SpeechToText, "
        "TextToSpeech, Vision, MapEncoding, SemanticRouter) with inputs, outputs, and a "
        "model_client. Set `trigger` to control when the component executes.\n"
        "4. **Wire Navigation** -- For mobile robots, configure a `RobotConfig` and "
        "instantiate Kompass components (Planner, Controller, DriveManager) with appropriate "
        "algorithms (DWA, PurePursuit, etc.).\n"
        "5. **Add Events & Fallbacks** -- Use `on_algorithm_fail()`, `on_component_fail()`, "
        "and custom event/action pairs for runtime adaptivity. Events can trigger model swaps, "
        "component restarts, or arbitrary callbacks.\n"
        "6. **Launch** -- Use `Launcher()` to add component packages and call `bringup()`. "
        "For production, run components in separate processes via `Launcher(multiprocessing=True)`.\n\n"
        "The documentation below is ordered as a curriculum: architecture first, then "
        "components and APIs, then example recipes of increasing complexity.\n\n"
        "---\n\n"
    )
    full_text.append(preamble)

    print(f"[llms.txt] Processing {len(LLMS_TXT_SELECTION)} files...")
    for relative_path in LLMS_TXT_SELECTION:
        file_path = src_dir / relative_path
        if file_path.exists():
            content = file_path.read_text(encoding="utf-8")
            full_text.append(format_for_llm(relative_path, content))
        else:
            print(f"[llms.txt] Warning: File not found: {relative_path}")

    output_path = out_dir / "llms.txt"
    try:
        output_path.write_text("".join(full_text), encoding="utf-8")
        print(f"[llms.txt] Successfully generated: {output_path}")
    except Exception as e:
        print(f"[llms.txt] Error writing file: {e}")


def create_robots_txt(app, exception):
    """Create robots.txt file for sitemap crawling."""
    if exception is not None:
        return
    dst_dir = app.outdir
    robots_path = os.path.join(dst_dir, "robots.txt")
    content = f"User-agent: *\n\nSitemap: {html_baseurl}sitemap.xml\n"
    with open(robots_path, "w") as f:
        f.write(content)


def setup(app):
    app.connect("build-finished", create_robots_txt)
    app.connect("build-finished", generate_llms_txt)
