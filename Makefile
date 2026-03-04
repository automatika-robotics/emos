.PHONY: docs docs-clean docs-serve

docs:
	bash build_docs.sh

docs-clean:
	rm -rf docs/_build

docs-serve: docs
	python -m http.server -d docs/_build/html 8000
