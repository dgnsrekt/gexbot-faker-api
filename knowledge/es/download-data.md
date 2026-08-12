---
type: Guide
title: Descargar datos
description: Las dos formas en que los datos de mercado llegan al faker — la ruta del EOD archive (daemon y CLI) y la ruta individual /hist (Studio) — más la GexBot API key que requieren.
tags: [download, eod, hist, api-key, gexbot, español]
timestamp: 2026-08-11T00:00:00Z
---

# Descargar datos

Descargar historia real desde GexBot requiere una **`GEXBOT_API_KEY`** con una
Quant Subscription. (Reproducir un día ya descargado no necesita clave — ver
[apuntar un cliente](point-a-client.md).) Los datos llegan al faker de dos formas,
y aterrizan en **estados diferentes de la Library**.

## Ruta 1 — reporte EOD (daemon por defecto + CLI)

GexBot sirve un zip por ticker pre-empaquetado (`GET /v2/hist/eod/{ticker}`) cuyos
miembros son los datasets por categoría comprimidos con gzip. El faker lo almacena
tal cual como el **archive comprimido canónico** bajo
`data/eod/YYYY-MM-DD/TICKER/`, más un sidecar `…zip.manifest.json` generado
localmente. Como es solo archive (todavía sin JSONL), la fecha muestra
**`archived`** y debe ser **Materialized** antes de poder ser **Loaded**. Esto
mantiene el uso de disco ligero.

Usa la CLI del downloader:

```bash
./bin/gexbot-downloader download 2025-11-14                     # single date
./bin/gexbot-downloader download 2025-11-01 2025-11-14          # date range
./bin/gexbot-downloader download --tickers SPX,NDX --packages state 2025-11-14
./bin/gexbot-downloader download --dry-run 2025-11-14           # preview
```

O el atajo de just: `just download-lookback 7` (últimos 7 días de mercado; los
fines de semana y feriados se omiten automáticamente).

## Ruta 2 — `/hist` individual (pantalla Download del Studio)

La pantalla **Download** del Studio obtiene JSON por categoría desde
`GET /v2/hist/{ticker}/{package}/{category}/{date}`, lo convierte automáticamente a
JSONL y empaqueta el mismo archive. El worker del Studio **también escribe el
marker `.eod-materialized`**, así que la fecha muestra **`ready` / Load`
inmediatamente** — sin un paso de Materialize aparte.

**La cobertura es autoritativa según el YAML.** La pantalla Download te deja
elegir solo **fechas** — los tickers, packages y categories provienen del **mismo
YAML del downloader que usa el daemon** (`GEXBOT_DOWNLOADER_CONFIG`, compartido con
`DAEMON_CONFIG_PATH`), mostrado en solo lectura con una etiqueta "Configured by …".
Así que una descarga manual del Studio cubre exactamente el mismo conjunto que el
daemon programado, y el servidor ignora cualquier ticker/package que envíe una
solicitud modificada. Para cambiar la cobertura, edita el YAML y reinicia/recarga
los servicios.

> El fallback fuera de horario del daemon usa esta misma ruta `/hist` pero
> **todavía no** escribe el marker, así que un fallback del daemon deja JSONL en
> disco mientras sigue mostrando `archived` / Materialize (registrado en
> [#38](https://github.com/dgnsrekt/gexbot-faker-api/issues/38)).

## Qué puedes descargar

- **Tickers** — Índices: SPX, NDX, RUT, VIX · ETFs: SPY, QQQ, IWM · Futuros:
  ES_SPX, NQ_NDX.
- **Packages / categories** — `state` (gex_full/zero/one + delta/gamma/vanna/charm
  en zero/one), `classic` (gex_full/zero/one), `orderflow` (orderflow).

Selecciona tickers, packages y categories en `configs/custom.yaml` (cópialo desde
`configs/default.yaml`). Este único YAML es la autoridad para **tanto** la
CLI/daemon **como** las descargas manuales del Studio — apunta
`GEXBOT_DOWNLOADER_CONFIG` (servidor) y `DAEMON_CONFIG_PATH` (daemon) al mismo
archivo para que nunca diverjan.

Siguiente: convierte un archive descargado en una fecha servida →
[materialize & load](materialize-load.md).
