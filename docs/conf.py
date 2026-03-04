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


# --- Post-build hooks ---

def copy_markdown_files(app, exception):
    """Copy source markdown files to build output."""
    if exception is not None:
        return
    src_dir = app.srcdir
    dst_dir = app.outdir
    for root, _, files in os.walk(src_dir):
        for file in files:
            if file.endswith(".md"):
                src_path = os.path.join(root, file)
                rel_path = os.path.relpath(src_path, src_dir)
                dst_path = os.path.join(dst_dir, rel_path)
                os.makedirs(os.path.dirname(dst_path), exist_ok=True)
                shutil.copy2(src_path, dst_path)


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
    app.connect("build-finished", copy_markdown_files)
    app.connect("build-finished", create_robots_txt)
