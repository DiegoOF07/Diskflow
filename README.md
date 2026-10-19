# DiskFlow

[![CI](https://github.com/DiegoOF07/Diskflow/actions/workflows/ci.yml/badge.svg)](https://github.com/DiegoOF07/Diskflow/actions/workflows/ci.yml)

Auditor visual de almacenamiento para espacios de trabajo de desarrollo. Analiza las carpetas que eliges y explica cómo se reparte su espacio (proyectos, dependencias, artefactos de compilación…) **sin modificar tus archivos**.

> Estado: en desarrollo (Fase 1). Plataforma inicial: Windows 10/11 x64.

## Stack

Go 1.27 · Wails v2 · Vue 3 + TypeScript · SQLite (`modernc.org/sqlite`).

## Desarrollo

Requisitos: Go 1.27+, Node 26+, pnpm 11+, [Wails CLI v2](https://wails.io/docs/gettingstarted/installation).

```powershell
wails dev     # app con recarga en caliente
wails build   # binario en build/bin/diskflow.exe
go test ./... # pruebas del backend
```

En un clon limpio, compila el frontend una vez (`wails build` o `pnpm --dir frontend build`) antes de `go vet ./...`: `main.go` embebe `frontend/dist`.

## Estructura

| Ruta | Contenido |
|---|---|
| `main.go` | Punto de entrada: embebe el frontend y llama a `desktop.Run` |
| `internal/desktop/` | Único paquete que conoce Wails (adaptador) |
| `frontend/` | UI Vue 3 + TS (Vite) |
| `docs/` | Decisiones de arquitectura (ADR) |

## Licencia

MIT
