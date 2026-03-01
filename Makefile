.PHONY: test coverage

EXCLUDE := examples

test:
	@PKGS=$$(go list ./... | grep -v /$(EXCLUDE)); \
	go test $$PKGS

coverage:
	@PKGS=$$(go list ./... | grep -v /$(EXCLUDE)); \
	go test -coverpkg=$$(echo $$PKGS | tr ' ' ',') \
		-coverprofile=coverage.out $$PKGS >/dev/null; \
	go tool cover -func=coverage.out | awk '/total/ {print $$3}'