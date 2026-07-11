.PHONY: fetch agent deb lint clean lab lab-destroy

# Package version; the release workflow overrides this from the git tag.
VERSION ?= 0.0.0-dev

# Target architecture: amd64 (default) or arm64. SR Linux ships multi-arch
# container images, so arm64 debs enable native labs on Apple Silicon.
ARCH ?= amd64
# NoPorts release tarballs use x64/arm64 naming
FETCH_ARCH := $(if $(filter arm64,$(ARCH)),arm64,x64)

GO_IMAGE ?= golang:1.25

# Download the pinned sshnpd release binaries (see SSHNPD_VERSION) into build/
fetch:
	ARCH=$(FETCH_ARCH) ./scripts/fetch-sshnpd.sh

# Build the NDK agent for SR Linux via Docker; no local Go needed.
# CI builds with setup-go instead (see .github/workflows/ci.yaml).
agent:
	docker run --rm -v $(CURDIR):/work -w /work/agent \
		-e CGO_ENABLED=0 -e GOOS=linux -e GOARCH=$(ARCH) \
		$(GO_IMAGE) go build -trimpath \
		-ldflags "-s -w -X main.version=$(VERSION)" \
		-o ../build/noports-agent .

# Build the .deb with nFPM (no local install needed, runs in Docker)
deb: build/sshnpd build/noports-agent
	docker run --rm -e VERSION=$(VERSION) -e ARCH=$(ARCH) \
		-v $(CURDIR):/tmp/pkg -w /tmp/pkg \
		goreleaser/nfpm package --config nfpm.yaml --packager deb --target build/

build/sshnpd:
	$(MAKE) fetch

build/noports-agent:
	$(MAKE) agent

lint:
	shellcheck opt/noports/*.sh scripts/*.sh
	pyang --strict yang/noports.yang

# Spin up / tear down the containerlab dev topology
lab:
	mkdir -p clab/secrets
	cd clab && sudo containerlab deploy -t noports-srl.clab.yml

lab-destroy:
	cd clab && sudo containerlab destroy -t noports-srl.clab.yml

clean:
	rm -rf build
