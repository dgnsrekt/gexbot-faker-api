---
type: Guide
title: Apunta un cliente al faker
description: Cómo reapuntar un cliente GexBot existente al faker — la base URL, el modelo de autenticación por header donde cualquier token no vacío funciona, y la reproducción secuencial por API key.
tags: [client, base-url, auth, playback, cursor, integration, español]
timestamp: 2026-08-11T00:00:00Z
---

# Apunta un cliente al faker

El faker replica las **rutas de datos principales y las formas de payload** de la
API real de GexBot, así que apuntar un cliente existente hacia él suele ser un
cambio de una sola línea. No es un espejo total — ver la nota de paridad en
[overview](overview.md) y la
[matriz de compatibilidad](https://github.com/dgnsrekt/gexbot-faker-api/blob/main/compatibility/matrix.json)
para conocer las diferencias (por ejemplo, `/tickers`).

## 1. Cambia la base URL

```
https://api.gexbot.com   →   http://localhost:8080
```

Por ejemplo, [quant-python-sockets](https://github.com/nfa-llc/quant-python-sockets)
solo necesita `BASE_URL = "http://localhost:8080"`. El origen desde el que se
accedió al Studio también es la base de la API, así que las URLs copiadas siguen
siendo correctas detrás de un reverse proxy.

## 2. Auth: cualquier token no vacío funciona

Las rutas de datos de mercado requieren un header `Authorization`, pero el faker
**nunca valida el token** — cualquier valor no vacío autentica:

```bash
export GEXBOT_API_KEY=test123   # the faker accepts any key
```

Se aceptan `Basic`, `Bearer` y un token pelado. Un header **faltante** en una
ruta de datos devuelve `400 {"error":"Authorization header not found."}`. El
descubrimiento de solo lectura (`/tickers`, `/health`, `/dates`, …) no
necesita header.

### Rutas de control y el token del Studio

El plano de control exclusivo del faker ([REST API](rest-api.md)) tiene dos rutas
**que mutan estado** — `load` y `reset` — que cambian el estado global del
servidor, así que están protegidas detrás del **token de auth del Studio**
(`STUDIO_AUTH_TOKEN`) **cuando hay uno configurado** (habitual en un faker
remoto/LAN): preséntalo como `Basic`/`Bearer` o recibes `401 {"error":"control
route requires the Studio auth token"}`. Un token **sin configurar** (dev local)
las deja abiertas. Las lecturas y el `seek` por cliente nunca están protegidos.
Desde la CLI, pasa el token con `--token` / `GEXFAKER_TOKEN` (ver
[gexfakercli](gexfakercli.md)).

## 3. Entiende la reproducción por key

El token no es solo una contraseña — **siembra un cursor de reproducción por
key**. Cada key distinta recorre los snapshots del día cargado de forma
independiente:

- Cada extracción de datos exitosa devuelve el snapshot **actual** y luego
  **avanza** el cursor de esa key en uno.
- Dos clientes con keys distintas reproducen a su propio ritmo; dos con la misma
  key comparten un cursor.
- `cache_mode=exhaust` (por defecto): después del último snapshot, las
  extracciones devuelven `404 {"error":"No more data available"}`.
- `cache_mode=rotation`: el cursor vuelve al inicio en lugar de dar 404.

Controla el cursor con `POST /reset` (rebobinar) y `POST /seek` (saltar a un
tiempo) — o vía [gexfakercli](gexfakercli.md) (`reset`, `seek`).

## 4. Reproducción de rango multi-día

Un cliente puede cargar un **span contiguo de días** como un único dataset entre
días con `POST /load` (`{from,to}` o `{dates[]}` — un solo `{date}` carga un día),
de modo que el cursor pasa del último snapshot de un día al siguiente en lugar de
terminar en un límite de sesión, y `seek` resuelve en cualquier punto del span
(con `in_gap`/`clamped` para los huecos nocturnos y los bordes del span). Ver
[gexfakercli](gexfakercli.md) (`load`, `current-load`, `coverage`) y
[materializar y cargar](materialize-load.md).

## Qué leer a continuación

- La lista completa de endpoints y la UI de Swagger → [REST API](rest-api.md).
- Streaming en vivo en lugar de polling → [WebSocket streaming](websockets.md).
- Un cliente listo para agentes → [gexfakercli](gexfakercli.md).
