---
type: Reference
title: Streaming por WebSocket
description: Los cinco hubs WebSocket del faker, el handshake /negotiate, la convención de nombres de grupos, el formato de frames protobuf/zstd de Azure Web PubSub y cómo conectar un cliente.
tags: [websocket, streaming, negotiate, hubs, protobuf, zstd, groups, español]
timestamp: 2026-08-11T00:00:00Z
---

# Streaming por WebSocket

El faker transmite datos en estilo en vivo sobre un protocolo WebSocket
**compatible con Azure Web PubSub** con frames de Protobuf + Zstd — el mismo
protocolo que producción. El detalle completo del protocolo (esquemas de mensajes,
pipeline de codificación) está en el `WEBSOCKET.md` del repo; esta es la
referencia de trabajo.

## Los cinco hubs

| Hub | Route | Datos |
| --- | --- | --- |
| `orderflow` | `/ws/orderflow` | DEX, GEX, convexidad, vanna, charm |
| `classic` | `/ws/classic` | Cadena GEX tradicional |
| `state_gex` | `/ws/state_gex` | Perfiles de GEX |
| `state_greeks_zero` | `/ws/state_greeks_zero` | Greeks (0DTE): delta, gamma, vanna, charm |
| `state_greeks_one` | `/ws/state_greeks_one` | Greeks (1DTE+) |

## Flujo de conexión

1. `GET /negotiate` con un header `Authorization` → devuelve las `websocket_urls`
   por hub y un **`prefix`** de grupo.
2. Conéctate a la URL de un hub con el subprotocolo `Sec-WebSocket-Protocol` →
   recibes un `ConnectedMessage` con un connection id.
3. Envía un `JoinGroupMessage` por cada grupo que quieras (construido con el
   prefix del paso 1) → recibes un `AckMessage`.
4. Recibes broadcasts de `DataMessage` en el intervalo configurado
   (`WS_STREAM_INTERVAL`).

## Nombres de grupos

Todos los grupos siguen **`{prefix}_{TICKER}_{hub_type}_{category}`**, donde
`prefix` viene de `/negotiate` (por defecto `blue`, definido por
`WS_GROUP_PREFIX`):

| Hub | Patrón de grupo | Ejemplo |
| --- | --- | --- |
| orderflow | `{prefix}_{TICKER}_orderflow_orderflow` | `blue_SPX_orderflow_orderflow` |
| classic | `{prefix}_{TICKER}_classic_{category}` | `blue_SPX_classic_gex_zero` |
| state_gex | `{prefix}_{TICKER}_state_{category}` | `blue_SPX_state_gex_zero` |
| greeks (0DTE) | `{prefix}_{TICKER}_state_{category}` | `blue_SPX_state_gamma_zero` |
| greeks (1DTE+) | `{prefix}_{TICKER}_state_{category}` | `blue_NDX_state_vanna_one` |

La pantalla **Live streams** de Studio tiene un constructor de nombres de grupo
que los arma por ti (consulta [Studio](studio.md)).

## Subprotocolos

| Subprotocolo | Formato de frame |
| --- | --- |
| `protobuf.webpubsub.azure.v1` | Protobuf binario (por defecto) |
| `json.reliable.webpubsub.azure.v1` | Texto JSON (reliable) |
| `json.webpubsub.azure.v1` | Texto JSON |

Los frames binarios envuelven un payload protobuf comprimido con Zstd dentro de un
`DataMessage`. Elige un subprotocolo `json.*` si prefieres parsear frames de texto.

## Keepalive y límites

El servidor envía ping cada 54s; el cliente debe responder con Pong dentro de 60s.
Buffer de envío de 256 mensajes por cliente, frame máximo de 512KB, timeout de
escritura de 10s.
