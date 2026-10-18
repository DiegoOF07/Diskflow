# ADR-003: Comunicación entre la UI y el motor

- **Estado:** Aceptado
- **Fecha:** 2026-10-17

## Contexto

La UI necesita iniciar y cancelar escaneos, recibir progreso y consultar el índice por páginas, sin acceder nunca al sistema de archivos ni a la base de datos.

## Decisión

Usar los **bindings y eventos de Wails** (ver ADR-001), detrás de una interfaz `Backend` en el frontend.

- **Comandos:** métodos exportados del adaptador `internal/desktop.App` (p. ej. `StartScan`, `CancelScan`, `Children`), con tipos TS generados.
- **Eventos:** `scan:progress`, como máximo 10 por segundo, y `scan:done`, con el resumen del escaneo.
- **Paginación:** las consultas siempre llevan límite; la UI nunca recibe el índice completo.
- **Desacople:** los componentes Vue dependen de `Backend`, no de `wailsjs`. Cambiar de transporte (Wails v3 u otro) solo afecta a `frontend/src/backend/`.

## Alternativa descartada

Un protocolo JSON lines propio sobre stdin/stdout (necesario con Tauri + sidecar). Exigía mantener tipos a mano y un correlador de peticiones.
