---
type: Tutorial
title: Inicio rápido — del clon a un stream en vivo
description: El camino dorado hacia un faker en marcha; clona, agrega una clave de GexBot, descarga un día con Docker, abre el Studio, carga la fecha y observa un stream de WebSocket en vivo.
tags: [quick-start, tutorial, docker, studio, getting-started, español]
timestamp: 2026-08-11T00:00:00Z
---

# Quick start

Objetivo: un faker en marcha reproduciendo un día de mercado real, con los datos
visibles en el Studio — en unos pocos minutos. Este es el camino dorado; cada
paso enlaza a detalle más profundo si lo necesitas.

## Requisitos previos

- [Go 1.24+](https://go.dev/doc/install) (`go version`)
- [just](https://github.com/casey/just#installation) (`just --version`)
- [Docker](https://docs.docker.com/get-docker/) (`docker --version`)
- Una [GexBot API key](https://www.gexbot.com/pricing) con una **Quant
  Subscription** — requerida solo para **descargar** historia, no para
  reproducirla.

## 1. Clona y agrega tu clave

```bash
git clone git@github.com:dgnsrekt/gexbot-faker-api.git
cd gexbot-faker-api
cp gexbot.example.env .env
# edit .env: set GEXBOT_API_KEY=your_key
```

## 2. Descarga un día de datos

```bash
just download-lookback 7   # last 7 market days (weekends/holidays skipped)
```

Esto almacena **EOD archives** comprimidos bajo `data/eod/YYYY-MM-DD/`. Ver
[download data](download-data.md) para otras formas de obtenerlos (una sola fecha,
tickers personalizados o la pantalla de apuntar y hacer clic del Studio).

## 3. Inicia el servidor

```bash
just up      # API server + daemon in Docker
just logs    # follow the logs
```

O ejecútalo localmente sin Docker: `just serve-gex-faker`.

## 4. Abre el Studio y carga la fecha

Abre **http://localhost:8080/studio**.

- Ve a **Data library** (biblioteca de datos). Una fecha recién descargada muestra
  **`archived`** — haz clic en **Materialize** para desempaquetarla y luego en
  **Load**. (¿Por qué dos pasos? Ver [materialize & load](materialize-load.md).)
- El encabezado muestra que el servidor ahora está reproduciendo esa fecha.

## 5. Observa un stream en vivo

Ve a **Live streams** (streams en vivo). Elige un ticker y un feed, copia el
nombre de grupo generado y conecta un cliente WebSocket — o simplemente observa
cómo se encienden los conteos de clientes del hub a medida que los consumidores se
suscriben. Ver [WebSocket streaming](websockets.md) para conectar un cliente.

## Primera victoria alcanzada

Ahora tienes un faker reproduciendo un día real. Lo que sigue:

- Extrae datos desde la línea de comandos o un agente → [gexfakercli](gexfakercli.md).
- Apunta un cliente existente hacia él → [apuntar un cliente](point-a-client.md).
- Explora cada pantalla → [Studio](studio.md).
