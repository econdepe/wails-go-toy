# README

## About

Go-toy is a toy Wails app whose purpose is to illustrate how to build a desktop app that can register a Go service as a background task in the OS, and interact with it.

The background task is a logger that writes to a file in the OS with a status update every ten seconds.

The GUI frontend is a Svelte app.

## Prerequisites

- **Go**: 1.23+ (see `go.mod`)
- **Node.js**: LTS recommended (Node 18+ is a safe default)
- **Wails CLI**: v2.11.0 (or compatible)

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@v2.11.0
```

### Linux (WebKit2GTK)

This project targets `webkit2gtk-4.1` (see `wails.json`).

```bash
sudo apt update
sudo apt install -y build-essential pkg-config libgtk-3-dev libwebkit2gtk-4.1-dev
```

### macOS (NOT READY YET)

```bash
xcode-select --install
```

### Windows (NOT READY YET)

- Microsoft Edge WebView2 Runtime
- MSVC Build Tools (Visual Studio / “Desktop development with C++”)

## Live Development

To run in live development mode, run `wails dev` in the project directory. This will run a Vite development
server that will provide very fast hot reload of your frontend changes. If you want to develop in a browser
and have access to your Go methods, there is also a dev server that runs on http://localhost:34115. Connect
to this in your browser, and you can call your Go code from devtools.

### Linux (WebKit2GTK)

If you are using a Linux distribution that does not have webkit2gtk-4.0 (such as Ubuntu 24.04), you will need to run `wails dev -tags webkit2_41`.

## Building

To build a redistributable, production mode package, use `wails build`.
