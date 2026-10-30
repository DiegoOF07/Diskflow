# DiskFlow

[![CI](https://github.com/DiegoOF07/Diskflow/actions/workflows/ci.yml/badge.svg)](https://github.com/DiegoOF07/Diskflow/actions/workflows/ci.yml)
[![Licencia: MIT](https://img.shields.io/badge/licencia-MIT-blue.svg)](LICENSE)

[English](README.md) · **Español**

DiskFlow es un auditor visual de almacenamiento para espacios de trabajo de desarrollo. Eliges una carpeta y muestra en qué se va el espacio (proyectos, dependencias, artefactos de compilación) con un treemap y una tabla ordenable, **sin modificar nunca tus archivos**.

> **Estado:** desarrollo temprano (`v0.2`). Por ahora solo para Windows 10/11 x64.

## Características

- **Carpetas guardadas.** La pantalla de inicio lista las carpetas que analizas, con su tamaño y hace cuánto se analizaron. Puedes renombrarlas o quitarlas de la lista sin tocar sus archivos.
- **Uso real del disco.** Los tamaños son lo que los archivos ocupan en disco, así que los archivos dispersos (como las imágenes de discos virtuales), los comprimidos con NTFS y los de OneDrive que solo están en la nube ya no inflan los totales. Cuando difiere, también se muestra el tamaño lógico.
- **Escaneo concurrente rápido**, con progreso en vivo y cancelación. El motor indexó ~100k entradas en unos 2,4 s sobre NTFS, con menos de 10 MB de heap.
- **Treemap y tabla lado a lado.** Al pasar el cursor por uno se resalta el elemento en el otro, y puedes entrar en cualquier carpeta. La tabla ordena por tamaño, nombre o fecha, carga las carpetas grandes por páginas y se usa con el teclado.
- **Búsqueda por nombre** en todo el índice, con los resultados más grandes primero; al elegir uno se abre su carpeta.
- **Mostrar en el Explorador y Copiar ruta** desde cada fila o su menú contextual.
- **Cobertura parcial a la vista.** Las carpetas que no se pudieron leer aparecen con el motivo, así sabes qué dejan fuera los totales. Si un escaneo falla o la unidad de una carpeta está desconectada, el escaneo anterior sigue disponible.
- **Índice local.** Los resultados se guardan en SQLite, así que puedes explorar el último escaneo sin repetirlo.
- **Solo lectura por diseño.** DiskFlow nunca borra, mueve ni abre tus archivos, y no hace peticiones de red.

## Instalación

Descarga la última versión desde [Releases](https://github.com/DiegoOF07/Diskflow/releases):

| Archivo | Descripción |
|---|---|
| `diskflow-<versión>-windows-amd64-installer.exe` | Instalador, que crea accesos directos en el menú Inicio y el escritorio (recomendado) |
| `diskflow-<versión>-windows-amd64.exe` | Ejecutable portable |
| `SHA256SUMS.txt` | Checksums para verificar las descargas |

Para comprobar que una descarga llegó íntegra, compara su hash con el que aparece en `SHA256SUMS.txt`:

```powershell
Get-FileHash .\diskflow-<versión>-windows-amd64-installer.exe -Algorithm SHA256
```

Las releases posteriores a `v0.1.0-rc.1` incluyen además una [atestación de procedencia](https://docs.github.com/actions/security-for-github-actions/using-artifact-attestations), que demuestra que el archivo lo compiló el workflow de release de este repositorio a partir de un commit concreto. Con [GitHub CLI](https://cli.github.com/) instalado:

```powershell
gh attestation verify .\diskflow-<versión>-windows-amd64-installer.exe --repo DiegoOF07/Diskflow
```

Los binarios todavía no están firmados, así que Windows SmartScreen mostrará un aviso la primera vez que los ejecutes, y el Control inteligente de aplicaciones puede bloquear el portable. Si ocurre, usa el instalador.

El índice de escaneos se guarda en `%LOCALAPPDATA%\DiskFlow\diskflow.db`.

## Limitaciones conocidas

- **Los hard links se cuentan una vez por enlace.** Por eso carpetas como `C:\Windows\WinSxS` pueden parecer más grandes que el espacio que ocupan de verdad.
- Solo se puede analizar una carpeta a la vez.
- La interfaz está pensada para ventanas de escritorio y aún no se adapta a pantallas pequeñas.
- Por ahora solo se admite Windows.

## Hoja de ruta

- **v0.3 (auditor de proyectos):** detectar proyectos (Node, Go, Python, Rust…), clasificar el espacio en categorías como dependencias y artefactos de compilación, y editar exclusiones.
- **Más adelante:** historial de escaneos y comparaciones, exportación a JSON/CSV, un diseño renovado que se adapte a pantallas más pequeñas e instalador firmado.

## Compilar desde el código

**Requisitos:** Go 1.27+, Node 26+, pnpm 11+ y [Wails CLI v2](https://wails.io/docs/gettingstarted/installation). Para generar el instalador también hace falta [NSIS](https://nsis.sourceforge.io/).

```powershell
wails dev                  # la app con recarga en caliente
wails build                # genera build/bin/diskflow.exe
go test ./...              # pruebas del backend
pnpm --dir frontend test   # pruebas del frontend (Vitest)
pnpm --dir frontend lint   # ESLint; `pnpm --dir frontend format` aplica Prettier
```

En un clon limpio, compila el frontend una vez (`wails build` o `pnpm --dir frontend build`) antes de ejecutar `go vet ./...`, porque `main.go` embebe `frontend/dist`.

### CLI de desarrollo

`diskflow-cli` prueba el motor de escaneo sin la interfaz. Los flags van antes de la ruta, y Ctrl+C cancela el escaneo.

```powershell
go run ./cmd/diskflow-cli gen --out C:\tmp\arbol --entries 100000   # genera un árbol sintético
go run ./cmd/diskflow-cli scan --db dev.db C:\tmp\arbol              # lo escanea y mide los tiempos
go run ./cmd/diskflow-cli top --db dev.db --limit 10 C:\tmp\arbol    # lista los directorios más grandes
```

Para saber en qué se va el tiempo de un escaneo, `scan` puede escribir perfiles de [pprof](https://pkg.go.dev/runtime/pprof). El perfil de bloqueos muestra las esperas en channels y mutex, y ralentiza el escaneo.

```powershell
go build -o diskflow-cli.exe ./cmd/diskflow-cli
.\diskflow-cli.exe scan --db dev.db --cpuprofile cpu.prof --memprofile mem.prof C:\tmp\arbol
go tool pprof -http=:8080 diskflow-cli.exe cpu.prof   # flame graph en el navegador
```

### Publicar una versión

Al subir un tag `v*` se ejecuta el workflow `release`, que compila en Windows y publica en GitHub Releases el ejecutable portable, el instalador y `SHA256SUMS.txt`. Los tags con sufijo (`v0.2.0-rc.1`) se publican como *prerelease*.

```powershell
git tag v0.2.0
git push origin v0.2.0
```

## Arquitectura

El motor de escaneo es Go puro, sin dependencias de la interfaz. Wails queda limitado a un único paquete adaptador, y el frontend se comunica con él a través de una interfaz `Backend`, lo que permite probar cada lado por separado. Las decisiones de diseño están registradas como [ADR](docs/README.md).

| Ruta | Contenido |
|---|---|
| `main.go` | Punto de entrada: embebe el frontend y llama a `desktop.Run` |
| `internal/engine/` | Motor de escaneo e índice SQLite, independientes de la UI |
| `internal/desktop/` | Adaptador de Wails, el único paquete que lo importa |
| `cmd/diskflow-cli/` | CLI de desarrollo sobre el motor |
| `frontend/src/backend/` | La interfaz `Backend` y su implementación sobre Wails |
| `frontend/src/features/` | Pantallas de la UI (Vue 3 + TypeScript, Vite) |
| `frontend/wailsjs/` | Bindings generados por Wails (no editar) |
| `docs/` | Registros de decisiones de arquitectura |

**Stack:** Go · Wails v2 · Vue 3 + TypeScript · SQLite (`modernc.org/sqlite`, Go puro) · d3-hierarchy.

## Licencia

[MIT](LICENSE) © 2026 Diego Oswaldo Flores Rivas
