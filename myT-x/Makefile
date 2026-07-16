.PHONY: build-shim prepare-embed dev build clean-embed

build-shim:
	go build -o tmux-shim.exe ./cmd/tmux-shim

prepare-embed: build-shim
	mkdir -p internal/install/embedded/shimbin
	cp -f tmux-shim.exe internal/install/embedded/shimbin/tmux-shim.exe

dev:
	go build -o tmux-shim.exe ./cmd/tmux-shim
	wails dev

build: prepare-embed
	wails build -tags embed_shim

clean-embed:
	rm -rf internal/install/embedded/shimbin
	rm -f tmux-shim.exe
