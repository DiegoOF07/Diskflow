# DiskFlow

[![CI](https://github.com/DiegoOF07/Diskflow/actions/workflows/ci.yml/badge.svg)](https://github.com/DiegoOF07/Diskflow/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

**English** · [Español](README.es.md)

DiskFlow is a visual storage auditor for development workspaces. Point it at a folder and it shows where the space goes (projects, dependencies, build artifacts) through a treemap and a sortable table, **without ever modifying your files**.

> **Status:** early development (`v0.2`). Windows 10/11 x64 only for now. The user interface is in Spanish.

## Features

- **Saved folders.** The home screen lists the folders you analyze, with their size and how long ago they were scanned. You can rename them or remove them from the list without touching their files.
- **Real disk usage.** Sizes are what files take on disk, so sparse files (such as virtual disk images), NTFS-compressed files and cloud-only OneDrive files no longer inflate the totals. The logical size is shown alongside when it differs.
- **Fast concurrent scanning** with live progress and cancellation. The engine indexed ~100k entries in about 2.4 s on NTFS while using under 10 MB of heap.
- **Treemap and table side by side.** Hovering in one highlights the item in the other, and you can drill down into any folder. The table sorts by size, name or date, loads large folders page by page and works from the keyboard.
- **Search by name** across the whole index, biggest results first; picking a result opens its folder.
- **Show in Explorer and Copy path** from each row or its context menu.
- **Partial coverage made visible.** Folders that could not be read are listed with the reason, so you know what the totals leave out. If a scan fails or a folder's drive is disconnected, the previous scan stays available.
- **Local index.** Results are stored in SQLite, so you can browse the last scan without rescanning.
- **Read-only by design.** DiskFlow never deletes, moves or opens your files, and it makes no network requests.

## Installation

Download the latest build from [Releases](https://github.com/DiegoOF07/Diskflow/releases):

| File | Description |
|---|---|
| `diskflow-<version>-windows-amd64-installer.exe` | Installer, which adds Start menu and desktop shortcuts (recommended) |
| `diskflow-<version>-windows-amd64.exe` | Portable executable |
| `SHA256SUMS.txt` | Checksums to verify the downloads |

To check that a download is intact, compare its hash with the one listed in `SHA256SUMS.txt`:

```powershell
Get-FileHash .\diskflow-<version>-windows-amd64-installer.exe -Algorithm SHA256
```

Releases after `v0.1.0-rc.1` also carry a [build provenance attestation](https://docs.github.com/actions/security-for-github-actions/using-artifact-attestations), which proves that a file was built by this repository's release workflow from a specific commit. With the [GitHub CLI](https://cli.github.com/) installed:

```powershell
gh attestation verify .\diskflow-<version>-windows-amd64-installer.exe --repo DiegoOF07/Diskflow
```

The binaries are not code-signed yet, so Windows SmartScreen will show a warning the first time you run them, and Smart App Control may block the portable executable. If that happens, use the installer.

The scan index is stored in `%LOCALAPPDATA%\DiskFlow\diskflow.db`.

## Known limitations

- **Hard links are counted once per link.** Folders such as `C:\Windows\WinSxS` can therefore look bigger than the space they really take.
- Only one folder can be scanned at a time.
- The interface is designed for desktop-sized windows and is not yet adapted to small screens.
- Only Windows is supported for now.

## Roadmap

- **v0.3 (project auditor):** detect projects (Node, Go, Python, Rust…), classify space into categories such as dependencies and build artifacts, and edit exclusions.
- **Later:** scan history and comparisons, export to JSON/CSV, a refreshed design that adapts to smaller screens, and a signed installer.

## Building from source

**Requirements:** Go 1.27+, Node 26+, pnpm 11+ and the [Wails CLI v2](https://wails.io/docs/gettingstarted/installation). Building the installer also needs [NSIS](https://nsis.sourceforge.io/).

```powershell
wails dev                  # run the app with hot reload
wails build                # build build/bin/diskflow.exe
go test ./...              # backend tests
pnpm --dir frontend test   # frontend tests (Vitest)
pnpm --dir frontend lint   # ESLint; `pnpm --dir frontend format` applies Prettier
```

In a fresh clone, build the frontend once (`wails build` or `pnpm --dir frontend build`) before running `go vet ./...`, because `main.go` embeds `frontend/dist`.

### Developer CLI

`diskflow-cli` exercises the scan engine without the UI. Flags go before the path, and Ctrl+C cancels a scan.

```powershell
go run ./cmd/diskflow-cli gen --out C:\tmp\tree --entries 100000   # generate a synthetic tree
go run ./cmd/diskflow-cli scan --db dev.db C:\tmp\tree              # scan it and report timings
go run ./cmd/diskflow-cli top --db dev.db --limit 10 C:\tmp\tree    # list the largest directories
```

To find out where a scan spends its time, `scan` can write [pprof](https://pkg.go.dev/runtime/pprof) profiles. The blocking profile shows waits on channels and mutexes, and it slows the scan down.

```powershell
go build -o diskflow-cli.exe ./cmd/diskflow-cli
.\diskflow-cli.exe scan --db dev.db --cpuprofile cpu.prof --memprofile mem.prof C:\tmp\tree
go tool pprof -http=:8080 diskflow-cli.exe cpu.prof   # flame graph in the browser
```

### Releasing

Pushing a `v*` tag runs the `release` workflow, which builds on Windows and publishes the portable executable, the installer and `SHA256SUMS.txt` to GitHub Releases. Tags with a suffix (`v0.2.0-rc.1`) are published as prereleases.

```powershell
git tag v0.2.0
git push origin v0.2.0
```

## Architecture

The scan engine is plain Go with no dependency on the UI. Wails is confined to a single adapter package, and the frontend talks to it through a `Backend` interface, which keeps both sides testable in isolation. The design decisions are recorded as [ADRs](docs/README.md), which are written in Spanish.

| Path | Contents |
|---|---|
| `main.go` | Entry point: embeds the frontend and calls `desktop.Run` |
| `internal/engine/` | Scan engine and SQLite index, independent of the UI |
| `internal/desktop/` | Wails adapter, the only package that imports Wails |
| `cmd/diskflow-cli/` | Developer CLI on top of the engine |
| `frontend/src/backend/` | The `Backend` interface and its Wails implementation |
| `frontend/src/features/` | UI screens (Vue 3 + TypeScript, Vite) |
| `frontend/wailsjs/` | Bindings generated by Wails (do not edit) |
| `docs/` | Architecture decision records |

**Stack:** Go · Wails v2 · Vue 3 + TypeScript · SQLite (`modernc.org/sqlite`, pure Go) · d3-hierarchy.

## License

[MIT](LICENSE) © 2026 Diego Oswaldo Flores Rivas
