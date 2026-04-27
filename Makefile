VERSION = v0.0.0-dev
ORIGIN = origin

.PHONY: test check-version release-github

test:
	go vet ./...
	go test -v -count=1 ./...

fmt:
	rg -t go . -l | xargs -i gofmt -w {}

# Pre-release sanity checks. Runs as a prerequisite of release targets, but is
# also usable standalone to dry-run the guards: `make check-version`.
check-version:
	@case "$(VERSION)" in *-dev) echo "VERSION is $(VERSION); bump it in the Makefile before releasing"; exit 1;; esac
	@echo "$(VERSION)" | grep -qE '^v[0-9]+\.[0-9]+\.[0-9]+(-[A-Za-z0-9.]+)?$$' \
		|| { echo "VERSION must look like v0.1.0 or v0.1.0-rc1"; exit 1; }
	@git diff --quiet HEAD \
		|| { echo "working tree is dirty; commit or stash first"; exit 1; }
	@if git rev-parse "$(VERSION)" >/dev/null 2>&1; then \
		echo "tag $(VERSION) already exists"; exit 1; \
	fi

# Tag HEAD and push the tag to GitHub; pushing v* triggers .github/workflows/release.yml.
# To release: bump VERSION above, commit the change, then run `make release-github`.
release-github: check-version test
	git tag -a $(VERSION) -m "Release $(VERSION)"
	git push $(ORIGIN) $(VERSION)

push-git-commits:
	git push $(ORIGIN) HEAD
	git push codeberg HEAD
