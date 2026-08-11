---
type: Reference
title: Referencia de la API REST
description: La superficie de endpoints REST del faker — descargas de datos de mercado, descubrimiento, control y estado de salud — más la interfaz interactiva Swagger UI en /docs y la especificación OpenAPI servida en /openapi.yaml.
tags: [rest, api, endpoints, openapi, swagger, reference, español]
timestamp: 2026-08-11T00:00:00Z
---

# Referencia de la API REST

La referencia completa y siempre actualizada es la **Swagger UI en `/docs`**
(http://localhost:8080/docs), generada a partir de la especificación OpenAPI
servida en **`/openapi.yaml`**. Esta página es el mapa; usa Swagger para los
esquemas exactos y para probar los endpoints.

## Descargas de datos de mercado (requiere header de auth; avanzan el cursor)

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
consulta [point a client](point-a-client.md) para el modelo de auth + cursor.

## Descubrimiento (sin auth)

| Endpoint | Devuelve |
| --- | --- |
| `GET /tickers` · `GET /tickers/quant` | Tickers disponibles |
| `GET /{package}/categories` | Categorías dentro de un package |
| `GET /available-dates` | Fechas materializadas listas para cargar |
| `GET /available-data/{date}` | Árbol de datos de una fecha (materializa bajo demanda) |
| `GET /current-date` | La fecha actualmente cargada |
| `GET /current-range` | El span actualmente cargado, en modo de rango multi-día |
| `GET /range-coverage?from=&to=` | Tickers por día + unión/intersección de un span (funciona antes de cargar) |
| `GET /load-range/status/{job_id}` | Progreso de una carga de rango asíncrona |
| `GET /health` | Estado, fecha cargada, modo de datos/caché |

## Control

Las rutas **que mutan estado** (`reload-date`, `reset-cache`, `load-range`)
requieren el **token de auth del Studio** — `STUDIO_AUTH_TOKEN`, presentado como
Basic/Bearer — **solo cuando el servidor tiene uno configurado** (si no, 401
`{"error":"control route requires the Studio auth token"}`); un token sin
configurar las deja abiertas (dev local). Las lecturas de arriba y
`seek-to-timestamp` (por cliente) nunca están protegidas. Ver
[apunta un cliente](point-a-client.md).

| Endpoint | Efecto |
| --- | --- |
| `POST /reload-date` `{date}` | Carga una sola fecha (materializa si hace falta); 409 si ya hay una recarga en curso · *protegido por token* |
| `POST /load-range` `{from,to}` o `{dates[]}` | Carga un span de días como un dataset continuo entre días (asíncrono → `job_id`) · *protegido por token* |
| `POST /reset-cache?key=` | Rebobina el cursor de una key (todas las keys si no se pasa `key`) · *protegido por token* |
| `POST /seek-to-timestamp` `{timestamp,key}` | Mueve una key a un timestamp unix; en modo rango resuelve a lo largo del span (`resolved_ts`, `day`, `in_gap`, `clamped`) |

## Relacionado

- `GET /negotiate` y los hubs `/ws/*` → [WebSocket streaming](websockets.md).
- `GET /sync/stream` (broadcast SSE de tiempo de mercado) → consulta la sección
  Sync Broadcast del README.
- Los endpoints de enlaces de descarga (`/download/{date}/{ticker}/...`) replican
  el layout histórico de GexBot.

Para los cuerpos de request/response y ejemplos, abre **`/docs`**.
