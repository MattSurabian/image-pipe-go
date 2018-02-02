SHELL := /bin/bash

# Determine the project name.
NAME := $(shell basename $(PWD))

# Get current version from file in repository.
VERSION := $(shell cat VERSION)

# Where to publish docker images.
IMAGE := docker.io/tkellen/$(NAME)

# Determine if repo is ready for releasing a build. This assumes a clean git
# repo with the exception of the VERSION file being modified.
IS_READY_TO_RELEASE := $(if $(strip $(shell git status --porcelain | grep -v VERSION 2>/dev/null)),yes,)

# Find the commit of the most recent release (for generating changelog).
LAST_RELEASE_SHA := $(shell git rev-list --tags --max-count=1)

# Find the sha of the first commit (for generating initial changelog).
FIRST_COMMIT_SHA := $(shell git rev-list --max-parents=0 HEAD)

# Determine if any releases have been made.
IS_FIRST_RELEASE := $(if $(LAST_RELEASE_SHA),no,yes)

# Determine if the version in the VERSION file is already tagged.
CURRENT_VERSION_IS_TAGGED := $(shell git rev-parse -q --verify $(VERSION))

# Build a git version range argument for all commits since the last release.
COMMIT_RANGE := $(if $(filter $(IS_FIRST_RELEASE),yes),,$(LAST_RELEASE_SHA)..HEAD)

# Get all changes in version range for producing a changelog.
COMMIT_HISTORY := $(shell git --no-pager log --format="%s (%h)" $(COMMIT_RANGE) --invert-grep --grep=no-changelog)

# Prepare the changelog in yaml format.
define CHANGELOG
$(VERSION):
  date: $(shell date +%Y-%m-%d)
  changes:
$(shell echo "$(COMMIT_HISTORY)" | sed 's/^/    - /')
endef
export CHANGELOG

# Configure version.
LD_FLAGS := "-ldflags "-X=main.Version=$(VERSION)"

# Source files.
SRC = $(shell find . -type f -name '*.go' -not -path "./vendor/*")

.PHONY: all fmt lint vet test run
all: run
fmt:
	@gofmt -l -w $(SRC)
lint:
	@golint $(SRC)
vet:
	@go vet $(SRC)
test:
	@go test -v $(SRC)

$(NAME): $(SRC)
	go build $(LDFLAGS) -o $(NAME)

build: fmt lint vet test $(NAME)
	@true

clean:
	@rm -f $(NAME)

run: build
	IMAGE_PIPE_HTTP_ADDR=:3000 IMAGE_PIPE_DEBUG=true ./$(NAME)

#build: Dockerfile src/*
#	docker build -t $(NAME) .
#
#run: build
#	docker run -it \
#		-e PORT \
#		-e AWS_ACCESS_KEY_ID \
#		-e AWS_SECRET_ACCESS_KEY \
#		$(NAME)

.PHONY: tag tag-latest tag-version
tag: tag-latest tag-version
tag-latest:
	docker tag $(NAME) $(IMAGE):latest
tag-version:
	docker tag $(NAME) $(IMAGE):$(VERSION)

.PHONY: publish publish-latest publish-version
publish: build publish-version publish-latest
publish-latest: tag-latest
	docker push $(IMAGE):latest
publish-version: tag-version
	docker push $(IMAGE):$(VERSION)

.PHONY: pre-release release post-release
pre-release:
	$(if $(IS_READY_TO_RELEASE),$(error Repo is not clean),)
	$(if $(CURRENT_VERSION_IS_TAGGED),$(error $(VERSION) is already tagged),)
	ex -sc "1i|$$CHANGELOG" -cx CHANGELOG
	git add -A && git commit -m "[no-changelog] $(VERSION)"
	git tag $(VERSION)
	git push origin master --tags

release: pre-release build publish post-release

post-release:
