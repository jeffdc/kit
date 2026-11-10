# Hard checks `mull done` runs (see `mull config`). Every tool with a
# Makefile is checked whatever changed, so finishing an item never
# leaves main red.

TOOLS := flint forage mull watchmen

.PHONY: done-checks
done-checks:
	@set -e; for t in $(TOOLS); do \
		echo "== $$t"; \
		$(MAKE) --no-print-directory -C $$t vet test; \
		if $(MAKE) -C $$t -n fmt-check >/dev/null 2>&1; then \
			$(MAKE) --no-print-directory -C $$t fmt-check; \
		fi; \
	done
