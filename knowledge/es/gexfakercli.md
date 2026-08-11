---
type: Guide
title: La CLI para agentes gexfakercli
description: Un cliente de línea de comandos JSON-first sobre el faker hecho para agentes LLM — el bootstrap de setup, el volcado de capacidades describe, las extracciones de datos, el cursor de reproducción y su instalación como skill de Claude/Codex.
tags: [gexfakercli, cli, agent, llm, setup, describe, cursor, español]
timestamp: 2026-08-11T00:00:00Z
---

# La CLI para agentes gexfakercli

`gexfakercli` (`cmd/gexfakercli`) es una **CLI JSON-first sobre el faker, hecha
para agentes LLM**. Cada comando imprime **un solo documento JSON en stdout**;
los errores y el progreso van a **stderr** como JSON; el código de salida es
distinto de cero en caso de fallo. Compílalo con `just build-gexfakercli`.

## Primer paso: llegar a un estado listo

```bash
gexfakercli setup
```

`setup` es un bootstrap de cero→listo: encuentra un faker en ejecución (o levanta
uno vía `docker compose`), garantiza que haya una fecha cargada descomprimiendo
un EOD archive en disco (**no se necesita API key** — la ruta sin key de
[materializar y cargar](materialize-load.md)), verifica con una extracción de
muestra e imprime el estado listo
(`{base_url, key, loaded_date, tickers, cache_mode, verified}`). Nunca descarga
sin `GEXBOT_API_KEY` y nunca se cuelga en silencio.

Luego conoce la superficie:

```bash
gexfakercli describe   # every command, endpoint, auth rule, and cursor rule as JSON
```

## El cursor de reproducción

Las extracciones de datos reproducen el día en orden; cada extracción avanza un
**cursor por `--key`** (key por defecto `gexfakercli`, nunca validada — ver
[apunta un cliente](point-a-client.md)).

- `cache_mode=exhaust` → `404 "No more data"` al final; `rotation` → vuelve al
  inicio.
- `gexfakercli reset` rebobina el cursor de la key activa (`--all` reinicia todas
  las keys).
- `gexfakercli seek <unix-ts>` salta al primer snapshot en/después de un
  timestamp.
- **Multi-día:** con un span cargado (`load --from/--to`, más abajo), el cursor
  pasa del último snapshot de un día al siguiente, y `seek` resuelve a lo largo de
  todo el span.

## Comandos comunes

```bash
gexfakercli status                      # is it up + which date is loaded
gexfakercli tickers                     # stocks/indexes/futures (--quant for the quant set)
gexfakercli dates                       # dates available to load
gexfakercli load 2026-07-17             # load a date (materializes if needed)

# data pulls (advance the cursor); --fields trims the payload for token thrift
gexfakercli classic SPX gex_zero --fields timestamp,spot,zero_gamma
gexfakercli state SPX gamma_zero
gexfakercli orderflow SPX
gexfakercli reset
```

Agregaciones: `gex_full|gex_zero|gex_one`. Tipos de state: `gex_*` y greeks
`delta|gamma|vanna|charm_zero` (0DTE) / `..._one` (1DTE+).

Control de salida: `--fields a,b,c`, `--pretty`, `--url` / `GEXFAKER_URL`
(por defecto `http://127.0.0.1:8080`), `--key` / `GEXFAKER_KEY` (el cursor de las
rutas de datos), `--token` / `GEXFAKER_TOKEN` (el token de auth del Studio para las
rutas de control protegidas).

## Reproducción multi-día

`load` maneja un solo día o un **span** contiguo — un span se sirve como un dataset
continuo para que la reproducción cruce los límites de día en lugar de morir en el
primer fin de sesión:

```bash
gexfakercli coverage --from 2026-08-06 --to 2026-08-10   # tickers por día + unión/intersección (antes de cargar)
gexfakercli load --from 2026-08-06 --to 2026-08-10        # materializa + carga el span (asíncrono; espera hasta terminar)
gexfakercli load --dates 2026-08-06,2026-08-07            # o una lista explícita
gexfakercli current-load                                 # confirma lo que está cargado
```

`load` es asíncrono ya sea un día o un span — sondea el job hasta completarlo
(progreso en stderr); `--no-wait` devuelve el id del job de inmediato y
`--timeout <sec>` acota la espera. Una vez cargado un span, `seek <unix-ts>`
resuelve a lo largo de él — la respuesta lleva `resolved_ts`, `day`, `in_gap`
(avanza a través de huecos nocturnos/fin de semana) y `clamped` (`start`/`end` en
los bordes del span; el tope final es `clamp` o `error` según el
`RANGE_END_POLICY` del servidor). Las extracciones de datos pasan entonces del
último snapshot de un día al siguiente.

## Auth de las rutas de control

Las rutas de control **que mutan estado** — `load` y `reset` — requieren el
**token de auth del Studio** cuando el servidor tiene uno configurado
(`STUDIO_AUTH_TOKEN`, p. ej. una máquina en la LAN). Preséntalo con `--token` o
`GEXFAKER_TOKEN` (enviado como Bearer); un `401` con el hint "requires the faker's
Studio auth token" significa que lo necesitas. Las rutas de solo lectura (`status`,
`dates`, `coverage`, `current-load`, `seek`, extracciones de datos) nunca lo
necesitan, y un token sin configurar significa que todo está abierto (dev local).
Ver [apunta un cliente](point-a-client.md).

## Instálala como skill de agente

El binario incrusta un `SKILL.md`; instálalo en Claude y/o Codex para que un
agente lo descubra automáticamente:

```bash
gexfakercli skill install            # both agents if their dirs exist
gexfakercli skill install --codex    # or target one
```

Esto escribe en `~/.claude/skills/gexfakercli/` y/o
`~/.codex/skills/gexfakercli/`.

## WebSocket streaming

El streaming en vivo (los cinco hubs, protobuf/zstd) existe en el servidor pero
**aún no está envuelto** por esta CLI — un fast-follow planeado. `describe` lista
los detalles de `/negotiate` bajo `websocket`. Para hacer streaming hoy, ver
[WebSocket streaming](websockets.md).
