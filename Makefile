name := recorder
hostname := $(shell hostnamectl --static 2>/dev/null || hostname -s)
host_config := hosts/$(hostname).json

.PHONY: install
install: install-dependencies install-tool install-config install-toggle

.PHONY: install-dependencies
install-dependencies:
	$(info [$(name)] Installing dependencies...)
	@sudo dnf install -y -q \
		kdotool \
		kdialog \
		pipewire-utils \
		pulseaudio-utils

.PHONY: install-tool
install-tool:
	$(info [$(name)] Building recorder...)
	@go build -o $(HOME)/.local/bin/recorder ./cmd/recorder

.PHONY: install-config
install-config: $(HOME)/.config/recorder/config.json

.PHONY: $(HOME)/.config/recorder/config.json
$(HOME)/.config/recorder/config.json:
	$(info [$(name)] Symlinking host config for $(hostname)...)
	@test -f "$(host_config)" || { echo "[$(name)] ERROR: missing $(host_config)" >&2; exit 1; }
	@mkdir -p $(dir $@)
	@ln -fsT $(abspath $(host_config)) $@

.PHONY: install-toggle
install-toggle: $(HOME)/.local/bin/recorder-toggle

$(HOME)/.local/bin/recorder-toggle: recorder-toggle
	$(info [$(name)] Installing toggle script...)
	@install -m 0755 $< $@

.PHONY: test
test:
	@go test ./...

.PHONY: build
build:
	@go build ./cmd/recorder
	@rm -f recorder
