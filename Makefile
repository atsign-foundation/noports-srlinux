.PHONY: fetch deb lint clean lab lab-destroy

# Package version; the release workflow overrides this from the git tag.
VERSION ?= 0.0.0-dev

# Download the pinned sshnpd release binary (see SSHNPD_VERSION) into build/
fetch:
	./scripts/fetch-sshnpd.sh

# Build the .deb with nFPM (no local install needed, runs in Docker)
deb: build/sshnpd
	docker run --rm -e VERSION=$(VERSION) -v $(CURDIR):/tmp/pkg -w /tmp/pkg \
		goreleaser/nfpm package --config nfpm.yaml --packager deb --target build/

build/sshnpd:
	./scripts/fetch-sshnpd.sh

lint:
	shellcheck opt/sshnpd/*.sh scripts/*.sh
	pyang --strict yang/noports-sshnpd.yang

# Spin up / tear down the containerlab dev topology
lab:
	mkdir -p clab/secrets
	cd clab && sudo containerlab deploy -t noports-srl.clab.yml

lab-destroy:
	cd clab && sudo containerlab destroy -t noports-srl.clab.yml

clean:
	rm -rf build
