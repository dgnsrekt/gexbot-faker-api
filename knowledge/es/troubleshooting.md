---
type: Guide
title: Solución de problemas
description: Soluciones para los problemas comunes del faker — sin datos para un ticker, una fecha atascada en archived vs ready, un error 400 de auth faltante, un cursor de reproducción agotado, la pantalla Download deshabilitada y bloqueos de localhost en sandboxes de agentes.
tags: [troubleshooting, errors, debugging, faq, español]
timestamp: 2026-08-11T00:00:00Z
---

# Solución de problemas

## "Data not found for TICKER/…"

La fecha cargada no tiene ese ticker/package/categoría. Revisa qué está realmente
cargado con `gexfakercli available <date>` (o `GET /available-data/{date}`). Un día
cargado solo contiene los tickers que descargaste — p. ej. puede tener QQQ pero no
SPX. Descarga el ticker que quieres y luego recarga.

## Una fecha muestra "Materialize", no "Load"

Es lo esperado para descargas de CLI/daemon: llegan como `archived` (solo el
archive comprimido). Haz clic en **Materialize** para desempacarlo y luego en
**Load** — o simplemente haz `load` (el servidor materializa bajo demanda).
Explicación completa: [materialize & load](materialize-load.md).

## Una fecha tiene JSONL en disco pero aún muestra "archived"

El fallback de `/hist` fuera de horario del daemon escribe JSONL sin el marker
`.eod-materialized` (issue #38). Vuelve a materializarla (Studio **Materialize**,
`gexbot-downloader eod materialize <date>`, o un `load`) para escribir el marker y
pasarla a `ready`.

## `400 {"error":"Authorization header not found."}`

Llegaste a una ruta de datos de mercado sin un header `Authorization`. Envía
cualquier token no vacío — el faker no lo valida (`export GEXBOT_API_KEY=test123`,
o `--key` en gexfakercli). Las rutas de descubrimiento/control no necesitan header.
Consulta [point a client](point-a-client.md).

## `404 {"error":"No more data available"}`

El cursor de reproducción de la key llegó al final del día en modo `exhaust`.
Rebobínalo con `gexfakercli reset` (o `POST /reset-cache`), o corre el servidor con
`CACHE_MODE=rotation` para hacer bucle en vez de 404.

## La pantalla Download dice "set GEXBOT_API_KEY"

Descargar historia real necesita una key. Define `GEXBOT_API_KEY` en el servidor y
reinicia. Reproducir datos ya descargados no necesita key.

## Logs / Monitoring muestran "unavailable"

Esas pantallas hacen proxy a Loki (`LOKI_URL`) y Prometheus (`PROMETHEUS_URL`), que
corren en el stack de docker. Correr el servidor con `go run` sin el stack los deja
sin definir, así que las pantallas degradan con un mensaje — es lo esperado. Usa
`just up` para el stack completo. Consulta
[docker & observability](docker-observability.md).

## La primera llamada `gexfakercli` de un agente falla con "operation not permitted"

Algunos sandboxes de agentes (p. ej. Codex) bloquean localhost por defecto; la
primera llamada a `127.0.0.1:8080` la deniega el sandbox, no el faker. Aprueba el
acceso a la red local y reintenta — el error estructurado distingue una denegación
del sandbox de que el servicio esté caído.

## Studio sin construir (corriendo `go run`)

`/studio` muestra una pista de "UI hasn't been built" si `web/dist` está vacío.
Corre `just studio-build` (o `just serve-gex-faker`, que lo construye) y reinicia.
Lo mismo aplica a `/guides` — corre `just docs-build`.
