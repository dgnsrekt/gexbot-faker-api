---
type: Reference
title: Referencia de la API REST
description: La superficie de endpoints REST del faker, dividida en rutas de paridad con GexBot real y el plano de control exclusivo del faker — descargas de datos de mercado, descubrimiento, las rutas de control de load/cursor y el estado de salud — más la interfaz interactiva Swagger UI en /docs y la especificación OpenAPI servida en /openapi.yaml.
tags: [rest, api, endpoints, openapi, swagger, parity, reference, español]
timestamp: 2026-08-11T00:00:00Z
---

# Referencia de la API REST

La referencia completa y siempre actualizada es la **Swagger UI en `/docs`**
(http://localhost:8080/docs), generada a partir de la especificación OpenAPI
servida en **`/openapi.yaml`**. Esta página es el mapa; usa Swagger para los
esquemas exactos y para probar los endpoints.

## Paridad vs exclusivo del faker

Cada endpoint es de uno de dos tipos — Swagger los etiqueta exactamente así:

- **Paridad con GexBot** — se comporta como la API real de GexBot. Un cliente
  escrito para GexBot funciona contra el faker sin cambios: mismas rutas, formas
  y auth por header. Son las descargas de datos de mercado y las rutas de
  descubrimiento/historia que replican producción.
- **Plano de control del faker** — **exclusivo del faker**. Estas manejan el mock
  — cargan datos de un día o un span, mueven el cursor de reproducción,
  inspeccionan lo que está cargado. **La API real de GexBot no tiene ninguna de
  ellas**; son la forma de operar el faker.

## Paridad con GexBot real

### Descargas de datos de mercado (requiere header de auth; avanzan el cursor)

| Endpoint | Devuelve |
| --- | --- |
| `GET /{ticker}/classic/{aggregation}` | Cadena Classic GEX (`gex_full`/`gex_zero`/`gex_one`) |
| `GET /{ticker}/classic/{aggregation}/majors` | Resumen de majors de Classic |
| `GET /{ticker}/classic/{aggregation}/maxchange` | Lookbacks de máximo cambio de GEX |
| `GET /{ticker}/state/{type}` | Perfil de State GEX o de greeks (`gex_*`, `delta/gamma/vanna/charm_zero|one`) |
| `GET /{ticker}/orderflow/orderflow` | Métricas de orderflow |
| `GET /options/{ticker}/expiries` | Expiries de opciones dentro del horizonte |
| `GET /futures/conversion` | Conversión afín futuros→índice |

Cada descarga exitosa devuelve el snapshot actual y avanza el cursor de esa key;
consulta [apunta un cliente](point-a-client.md) para el modelo de auth + cursor.

### Descubrimiento (sin auth)

| Endpoint | Devuelve |
| --- | --- |
| `GET /tickers` · `GET /tickers/quant` | Tickers disponibles |
| `GET /{package}/categories` | Categorías dentro de un package |

Las rutas de descarga histórica (`GET /hist/...`, `GET /download/{date}/{ticker}/...`)
también replican el layout de GexBot.

## Plano de control del faker

**Exclusivo del faker** — no existe en la API real de GexBot. El vocabulario
coincide 1:1 con los subcomandos de [gexfakercli](gexfakercli.md).

Las rutas que mutan estado (`load`, `reset`) requieren el **token de auth del
Studio** — `STUDIO_AUTH_TOKEN`, presentado como Basic/Bearer — **solo cuando el
servidor tiene uno configurado** (si no, 401 `{"error":"control route requires the
Studio auth token"}`); un token sin configurar las deja abiertas (dev local). Las
lecturas de abajo y `seek` (por cliente) nunca están protegidas. Ver
[apunta un cliente](point-a-client.md).

### Cargar e inspeccionar

| Endpoint | Efecto |
| --- | --- |
| `POST /load` `{date}` \| `{from,to}` \| `{dates[]}` | Carga un día o un span como un dataset continuo entre días. Asíncrono → `{job_id}`; un solo día = span de 1 · *protegido por token* |
| `GET /load/status/{job_id}` | Progreso de una carga asíncrona |
| `GET /current-load` | Qué está cargado: `dates[]`, `from`, `to`, `files_loaded`, `loaded_at` |
| `GET /dates` | Fechas materializadas listas para cargar |
| `GET /available/{date}` | Árbol de datos de una fecha (materializa bajo demanda) |
| `GET /coverage?from=&to=` | Tickers por día + unión/intersección de un span (funciona antes de cargar) |

`POST /load` es **asíncrono uniforme**: siempre devuelve un `job_id`; sondea
`/load/status/{job_id}` hasta `state=done` (una carga de un solo día simplemente
termina rápido).

### Cursor de reproducción

| Endpoint | Efecto |
| --- | --- |
| `POST /reset?key=` | Rebobina el cursor de una key (todas las keys si no se pasa `key`) · *protegido por token* |
| `POST /seek` `{timestamp,key}` | Mueve una key a un timestamp unix; en modo rango resuelve a lo largo del span (`resolved_ts`, `day`, `in_gap`, `clamped`) |

### Estado de salud

| Endpoint | Devuelve |
| --- | --- |
| `GET /health` | Estado, fecha cargada, modo de datos/caché |

## Relacionado

- `GET /negotiate` y los hubs `/ws/*` → [WebSocket streaming](websockets.md).
- `GET /sync/stream` (broadcast SSE de tiempo de mercado) → consulta la sección
  Sync Broadcast del README.

Para los cuerpos de request/response y ejemplos, abre **`/docs`**.
