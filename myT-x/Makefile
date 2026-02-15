.PHONY: build-shim prepare-embed dev build clean-embed

build-shim:
	go build -o tmux-shim.exe ./cmd/tmux-shim

prepare-embed: build-shim
	if not exist internal\install\embedded\shimbin mkdir internal\install\embedded\shimbin
	copy /Y tmux-shim.exe internal\install\embedded\shimbin\tmux-shim.exe

dev:
	go build -o tmux-shim.exe ./cmd/tmux-shim
	wails dev

build: prepare-embed
	wails build -tags embed_shim

clean-embed:
	if exist internal\install\embedded\shimbin rmdir /S /Q internal\install\embedded\shimbin
	if exist tmux-shim.exe del tmux-shim.exe
