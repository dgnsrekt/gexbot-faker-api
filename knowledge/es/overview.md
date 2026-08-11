---
type: Overview
title: Qué es el GEX Faker
description: Un servidor en Go que reproduce datos históricos de mercado de opciones/GEX de GexBot sobre REST y WebSocket, replicando las rutas de datos principales de la API real, además de un downloader, un daemon de programación, una interfaz web y una CLI para agentes.
tags: [overview, faker, gexbot, rest, websocket, replay, español]
timestamp: 2026-08-11T00:00:00Z
---

# Qué es el GEX Faker

El **GEX Faker API** es un servidor en Go que **reproduce datos históricos de
mercado de GexBot** — exposición a gamma de opciones (GEX), perfiles de griegas y
orderflow — sobre rutas REST y WebSocket que replican la superficie de datos
principal de la API real de [GexBot](https://www.gexbot.com). Apunta un cliente,
dashboard o herramienta de trading al faker en lugar de producción y este
reproduce un día de mercado grabado, snapshot por snapshot, sin consumir cuota de
la API en vivo y con salida totalmente determinista.

> **Paridad, con honestidad.** Las rutas de datos de mercado centrales y las
> formas de los payloads coinciden con producción, así que un cliente construido
> contra ellas funciona contra GexBot. No es un espejo total: algunos endpoints
> difieren (p. ej. `/tickers` devuelve los tickers *cargados*, no el universo
> soportado en vivo), y los hubs de WebSocket son los cinco hubs de replay del
> faker. Las diferencias conocidas se registran en
> [`compatibility/matrix.json`](https://github.com/dgnsrekt/gexbot-faker-api/blob/main/compatibility/matrix.json)
> y en la [auditoría de compatibilidad en vivo](https://github.com/dgnsrekt/gexbot-faker-api/blob/main/docs/GEXBOT_LIVE_COMPATIBILITY_AUDIT.md).

## Por qué existe

- **Desarrolla y prueba sin conexión.** Construye contra un feed de datos estable
  y repetible en lugar del mercado en vivo. El mismo día se reproduce de forma
  idéntica en cada ejecución.
- **No necesitas una clave de producción para *servir*.** Descargar historia real
  requiere una clave Quant de GexBot, pero una vez que un día está en disco el
  faker lo reproduce para cualquier cliente y acepta **cualquier** token (ver
  [apuntar un cliente](point-a-client.md)).
- **Paridad con producción.** Las rutas REST, los hubs de WebSocket y las formas
  de los payloads replican la API real, así que un cliente que funciona contra el
  faker funciona contra GexBot.

## Las piezas

| Pieza | Qué es |
| --- | --- |
| **API server** (`cmd/server`) | Sirve REST + WebSocket + la interfaz de Studio; reproduce una fecha cargada |
| **Studio** | Interfaz web embebida en `/studio` para descargar, cargar y observar datos sin curl |
| **Downloader** (`cmd/downloader`) | CLI que obtiene días históricos de GexBot hacia archives locales |
| **Daemon** (`cmd/daemon`) | Programador de larga duración que descarga automáticamente cada día de mercado |
| **gexfakercli** (`cmd/gexfakercli`) | CLI orientada a JSON pensada para que agentes LLM manejen el faker |

## Cómo fluyen los datos

1. **Download** de un día de mercado desde GexBot (CLI, daemon o el Studio) →
   aterriza como un **EOD archive** comprimido en disco.
2. **Materialize** del archive → archivos JSONL por categoría que el servidor puede
   reproducir.
3. **Load** de una fecha → el servidor sirve sus snapshots sobre REST y WebSocket.
4. Un cliente **extrae** datos; cada API key recorre los snapshots del día en
   orden.

El ciclo de vida download → materialize → load es lo único que vale la pena
entender de entrada — ver [materialize & load](materialize-load.md). Para poner
todo en marcha ahora, ve al [quick start](quick-start.md).
