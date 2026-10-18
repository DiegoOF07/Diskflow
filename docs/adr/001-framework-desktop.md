# ADR-001: Framework de escritorio

- **Estado:** Aceptado
- **Fecha:** 2026-10-17

## Contexto

DiskFlow es una aplicación de escritorio para Windows con una interfaz Vue 3 + TypeScript y un motor de escaneo en Go. Se evaluaron dos opciones con el mismo motor y la misma UI, cambiando solo el adaptador de escritorio:

- **Tauri 2 + sidecar Go:** Rust lanza el motor como proceso aparte y se comunica con él por JSON lines sobre stdin/stdout.
- **Wails v2:** el motor corre dentro del proceso Go de la aplicación; Wails genera los bindings TypeScript y ofrece un sistema de eventos.

## Comparación (prototipo, Windows)

| Criterio | Tauri 2 + sidecar | Wails v2 |
|---|---|---|
| Código de integración | ≈350 líneas (Rust, protocolo propio, correlador TS) | ≈100 líneas |
| Tipos entre Go y TS | Manuales | Generados desde los structs Go |
| Aislamiento de fallos | Un fallo del motor no tumba la UI | Un solo proceso |
| Build (en frío / incremental) | ~380 s / ~180 s | ~35 s / ~35 s |
| Toolchains | Go + Rust + Node | Go + Node |
| Memoria en reposo | Similar (~360 MB de working set, en su mayoría WebView2) | Similar |
| Madurez | Tauri 2 estable | Wails v2 estable; v3 en beta |

## Decisión

Usar **Wails v2**: menos piezas que mantener, tipos generados y ciclos de build cortos.

Para acotar la futura migración a Wails v3:

- Todo import de `github.com/wailsapp/wails/v2` vive en `internal/desktop`. `main.go` solo embebe el frontend compilado.
- El motor (`internal/engine`) no conoce Wails.
- La UI usa una interfaz `Backend`; solo `frontend/src/backend/` conoce los bindings generados (`wailsjs`).

## Consecuencias

- Wails v2.13+ exige Go ≥ 1.25.
- Un pánico del motor cierra la aplicación: el motor debe manejar los errores por entrada sin abortar el escaneo.
- La migración a v3 afectará a `internal/desktop`, `main.go`, `frontend/src/backend/` y `wails.json`.
