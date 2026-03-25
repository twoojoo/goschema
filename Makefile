.PHONY: test coverage bump-patch bump-minor bump-major

EXCLUDE := examples

test:
	@PKGS=$$(go list ./... | grep -v /$(EXCLUDE)); \
	go test $$PKGS

coverage:
	@PKGS=$$(go list ./... | grep -v /$(EXCLUDE)); \
	go test -coverpkg=$$(echo $$PKGS | tr ' ' ',') \
		-coverprofile=coverage.out $$PKGS > /dev/null; \
	go tool cover -func=coverage.out | awk '/total/ {print $$3}'

# ── Semver bump helpers ────────────────────────────────────────────────────────
# Reads the latest git tag (defaults to v0.0.0 if none exists), increments
# the requested segment, creates the tag, and pushes it to origin.
# Pushing the tag triggers the release.yml GitHub Actions workflow.

_LATEST = $(shell git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")
_MAJOR  = $(shell echo $(_LATEST) | sed 's/v//' | cut -d. -f1)
_MINOR  = $(shell echo $(_LATEST) | sed 's/v//' | cut -d. -f2)
_PATCH  = $(shell echo $(_LATEST) | sed 's/v//' | cut -d. -f3)

release-patch:
	$(eval NEXT := v$(_MAJOR).$(_MINOR).$(shell echo $$(($(_PATCH)+1))))
	@echo "Tagging: $(_LATEST) → $(NEXT)"
	@git tag $(NEXT) && git push origin $(NEXT)

release-minor:
	$(eval NEXT := v$(_MAJOR).$(shell echo $$(($(_MINOR)+1))).0)
	@echo "Tagging: $(_LATEST) → $(NEXT)"
	@git tag $(NEXT) && git push origin $(NEXT)

release-major:
	$(eval NEXT := v$(shell echo $$(($(_MAJOR)+1))).0.0)
	@echo "Tagging: $(_LATEST) → $(NEXT)"
	@git tag $(NEXT) && git push origin $(NEXT)