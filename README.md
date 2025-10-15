# jdash

*A terminal dashboard for Jenkins*

![Status: Alpha](https://img.shields.io/badge/status-alpha-orange)
![Go 1.25+](https://img.shields.io/badge/go-1.21+-blue)

<!-- Demo GIF/screenshot will go here when available -->

## What is jdash?

`jdash` is a terminal UI for Jenkins that brings your CI/CD workflow into the command line. Inspired by [lazygit](https://github.com/jesseduffield/lazygit) and [lazydocker](https://github.com/jesseduffield/lazydocker), it provides a fast, keyboard-driven interface for managing Jenkins jobs without leaving your terminal.

Built with Go and [Bubbletea](https://github.com/charmbracelet/bubbletea), `jdash` is faster than the web UI and keeps you in your workflow.

## Features

- 🌳 **Hierarchical job tree** — Navigate your Jenkins jobs in a folder structure
- ⚡️ **Real-time updates** — Live build queue
- 📜 **Console logs** — Stream build logs directly in your terminal
- 🔍 **Fuzzy search** — Find jobs instantly as you type
- ⌨️ **Vim-style navigation** — `hjkl` movement, `/` search, familiar keybindings
- 🎯 **Parameterized builds** — Trigger builds with custom parameters
- 🎨 **Clean interface** — Multi-panel layout with color-coded status

## Installation

### From source

```bash
git clone https://github.com/gorbach/jdash.git
cd jdash
go build -o jdash
```

**Prerequisites:** Go 1.25 or higher

## Getting Started

Run `jdash` for the first time:

```bash
./jdash
```

You'll be prompted to enter:
- Jenkins server URL
- Username
- API token (generate from Jenkins → User → Configure → API Token)

After successful authentication, your config is saved to `~/.jdash/config.json` and you won't need to authenticate again.

## Keyboard Navigation

### Global
- `Tab` / `Shift+Tab` — Cycle through panels
- `1` / `2` / `3` — Jump to specific panel
- `r` — Refresh all data
- `?` — Show help overlay
- `q` / `Ctrl+c` — Quit

### Jobs List (Panel 1)
- `j` / `k` or `↑` / `↓` — Navigate up/down
- `h` / `l` or `←` / `→` — Collapse/expand folders
- `Space` — Toggle folder
- `Enter` — View job details
- `g` / `G` — Jump to top/bottom
- `/` — Fuzzy search
- `Esc` — Clear search

### Actions
- `b` — Build now
- `l` — View console logs
- `a` — Abort running build
- `p` — Build with parameters

## Configuration

Config location: `~/.jdash/config.json`

```json
{
  "server": {
    "url": "https://jenkins.example.com",
    "username": "your-username",
    "token": "your-api-token"
  }
}
```

To reset authentication, delete this file and restart `jdash`.

## Project Status

`jdash` is in active development. Currently implemented:

- ✅ Authentication screen
- ✅ Multi-panel layout
- ✅ Jobs list with hierarchical tree
- ✅ Vim-style navigation
- ✅ Fuzzy search
- ✅ Real-time build queue
- ✅ Job details view
- ✅ Console log viewer
- ✅ Build triggering (basic and parameterized)
- ✅ Status bar with server info

Planned features:

- 🔄 Build history view
- 🔄 Color schemes

## Under the Hood

`jdash` uses:

- [Bubbletea](https://github.com/charmbracelet/bubbletea) — The Elm Architecture for terminal UIs
- [Bubbles](https://github.com/charmbracelet/bubbles) — TUI components (list, viewport, spinner, textinput)
- [Lipgloss](https://github.com/charmbracelet/lipgloss) — Styling and layout

## Contributing

Contributions are welcome! Please check the [issues](https://github.com/gorbach/jdash/issues).

To contribute:
1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Submit a pull request

## License

MIT License — see [LICENSE](LICENSE) for details.

## Author

Created by Oleksii Gorbach [@gorbach](https://github.com/gorbach)

---

*Built with ❤️ and [Bubbletea](https://github.com/charmbracelet/bubbletea)*
