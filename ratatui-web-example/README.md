# Ratatui Web Example

A minimal example of running a **Ratatui TUI** as a web app using **Ratzilla** + **WebAssembly**.

## What's happening

1. Rust code uses normal Ratatui widgets (`Paragraph`, `Table`, `Block`...)
2. **Ratzilla** provides a `DomBackend` that renders each frame as HTML DOM elements
3. **Trunk** compiles the Rust to WASM and bundles it into a single HTML page

## Setup & run

```bash
# 1. Install required tools
rustup target add wasm32-unknown-unknown
cargo install --locked trunk

# 2. Build and serve
trunk serve
```

Then open `http://localhost:8080` in your browser.

## Controls

| Key | Action |
|-----|--------|
| ↑ / k | Move selection up |
| ↓ / j | Move selection down |
| q | Quit (close tab) |

## Architecture

```
Browser (HTML/CSS)
  └── <div id="app">          ← Ratzilla renders widgets here as DOM nodes
        ├── Table (header + rows)
        ├── Paragraph (title bar)
        └── ... each widget = real HTML elements
```

Unlike ANSI-based approaches (tui2web, ratatui-wasm-backend), **Ratzilla** converts each frame into actual HTML elements — so you get native DOM rendering with full CSS/JS interop.
