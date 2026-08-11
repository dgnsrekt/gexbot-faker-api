---
type: Explanation
title: El ciclo de vida de materializar y cargar
description: Por qué una fecha que acabas de descargar muestra Materialize en lugar de Load — el ciclo de vida archived a ready a loaded, los tres estados de la Data library y por qué el archive comprimido es la fuente de verdad duradera.
tags: [materialize, load, lifecycle, archived, ready, loaded, jsonl, español]
timestamp: 2026-08-11T00:00:00Z
---

# El ciclo de vida de materializar y cargar

Este es el punto de confusión más común: **una fecha que acabas de descargar
suele mostrar "Materialize", no "Load".** Aquí explicamos por qué, y qué
significa cada estado.

## Dos formas en disco

Un día de mercado existe en disco en una o en ambas de estas dos formas:

1. **EOD archive** — un zip comprimido por ticker en
   `data/eod/YYYY-MM-DD/TICKER/`. Compacto, duradero, la **fuente de verdad
   canónica**. Esto es lo que producen el daemon y el downloader CLI.
2. **JSONL materializado** — archivos sin comprimir por categoría en
   `data/YYYY-MM-DD/TICKER/PACKAGE/CATEGORY.jsonl`, con un marker
   `.eod-materialized`. Esto es lo que el servidor realmente **reproduce**. El
   JSONL está sin comprimir porque la reproducción por stream necesita offsets
   de registro que gzip no puede proporcionar de forma eficiente.

**Materialize** = descomprimir los miembros gzip del archive en JSONL y escribir
el marker. **Load** = comenzar a servir ese JSONL a través de la API.

## Los tres estados de la Data library

| Estado | Botón | Significado |
| --- | --- | --- |
| `archived` | **Materialize** | Solo el EOD archive está en disco (lo típico en descargas de daemon/CLI) — primero descomprímelo |
| `ready` | **Load** | El JSONL está materializado en disco — cargar es instantáneo |
| `loaded` | *Loaded* | Actualmente lo está sirviendo la API |

Así que el flujo es **archived → (Materialize) → ready → (Load) → loaded**. Una
fecha descargada vía la CLI o el daemon comienza en `archived`; una fecha
descargada vía la pantalla `/hist` del Studio salta directo a `ready` porque esa
ruta escribe el marker por ti (ver [download data](download-data.md)).

## Materialize bajo demanda

No siempre tienes que hacer clic en Materialize. El servidor **materializa una
fecha bajo demanda** cuando carga una que aún no está descomprimida — incluyendo
vía `POST /reload-date` y `gexfakercli load <date>`. Materialize en la Data
library es simplemente una forma de hacer la descompresión con anticipación (y en
segundo plano) para que un Load posterior sea instantáneo.

## Limpieza por TTL (opcional, desactivada por defecto)

Si `GEXBOT_OUTPUT_AUTO_CLEANUP=true` (ventana `GEXBOT_OUTPUT_CLEANUP_AFTER_DAYS`),
el daemon expulsa el JSONL materializado inactivo dejándolo solo como archive
después de la ventana, recuperando disco. El archive no se toca — una fecha
siempre se puede volver a materializar. Esto está **desactivado por defecto**.

## Rango multi-día

Cargar no se limita a un solo día. Un **span** de días contiguos puede cargarse
como un único dataset entre días (`POST /load-range`, o `gexfakercli load-range` —
materializa primero los días archivados del span y luego los carga juntos). El
cursor de reproducción pasa entonces del último snapshot de un día directo al
siguiente, y `seek` resuelve en cualquier punto del span. Ver
[gexfakercli](gexfakercli.md) y [apunta un cliente](point-a-client.md).

## Por qué el archive es la fuente de verdad

El zip es pequeño y canónico; el JSONL es una caché de reproducción derivada y
regenerable. Mantener el archive como la forma duradera (y materializar bajo
demanda) es lo que permite al faker conservar cientos de días de historia sin el
costo en disco de mantener cada día descomprimido.

Si una fecha no carga o muestra el estado incorrecto, ver
[troubleshooting](troubleshooting.md).
