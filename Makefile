.PHONY: docs docs-deps docs-strict docs-clean docs-serve

DOCS_SRC   := docs
DOCS_OUT   := docs/_build/html
DOCS_PORT  ?= 8000

# Default build: ensures submodules are present (autodoc2 reads from them)
# then runs sphinx. Use `make docs-strict` in CI to fail on warnings.
docs:
	@echo "==> Updating submodules (autodoc2 sources)..."
	git submodule update --init --recursive
	@echo "==> Building Sphinx HTML..."
	sphinx-build -b html $(DOCS_SRC) $(DOCS_OUT)
	@echo ""
	@echo "==> Done. Open $(DOCS_OUT)/index.html"
	@echo "    or run: make docs-serve"

# `pip install -r docs/requirements.txt` in one line so contributors don't
# have to remember the path.
docs-deps:
	pip install -r $(DOCS_SRC)/requirements.txt

# Treat warnings as errors. Used by CI to catch broken cross-references,
# missing toctree entries, etc.
docs-strict:
	git submodule update --init --recursive
	sphinx-build -b html -W --keep-going $(DOCS_SRC) $(DOCS_OUT)

docs-clean:
	rm -rf $(DOCS_OUT)

docs-serve: docs
	python3 -m http.server -d $(DOCS_OUT) $(DOCS_PORT)
