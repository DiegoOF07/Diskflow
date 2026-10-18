# ADR-002: SQLite con `modernc.org/sqlite`

- **Estado:** Aceptado
- **Fecha:** 2026-10-17

## Contexto

El motor necesita persistencia local sin servidor para el índice de escaneo, la configuración y el historial. `mattn/go-sqlite3` requiere CGO, que en Windows obliga a instalar un compilador de C (MinGW). `modernc.org/sqlite` es una traducción a Go puro.

## Decisión

Usar `modernc.org/sqlite` con `database/sql` y **sin ORM**, con consultas SQL explícitas.

- Modo WAL, `busy_timeout` y un solo escritor (`SetMaxOpenConns(1)`).
- Inserciones en lotes dentro de transacciones.
- Migraciones numeradas, embebidas en el binario y controladas por `PRAGMA user_version`.

## Evidencia (prototipo, Windows)

- 500 000 entradas indexadas en unos 22–32 s, con menos de 45 MB de heap y una base de datos de unos 27 MB.
- Compila sin CGO en Windows y en Linux.

## Consecuencias

- Con un solo escritor, más workers de escaneo apenas mejoran el rendimiento. Cualquier optimización (tamaño de lote, `synchronous=NORMAL`, crear índices al final) debe ir respaldada por perfiles.
- Las versiones nuevas del driver suben su versión mínima de Go: al actualizarlo hay que revisar la directiva `go` de `go.mod`.
