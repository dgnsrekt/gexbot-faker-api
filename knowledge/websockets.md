---
type: Reference
title: WebSocket streaming
description: The faker's five WebSocket hubs, the /negotiate handshake, the group-naming convention, the Azure Web PubSub protobuf/zstd frame format, and how to connect a client.
tags: [websocket, streaming, negotiate, hubs, protobuf, zstd, groups]
timestamp: 2026-08-11T00:00:00Z
---

# WebSocket streaming

The faker streams live-style data over an **Azure Web PubSub-compatible**
WebSocket protocol with Protobuf + Zstd frames — the same protocol as production.
Full protocol detail (message schemas, encoding pipeline) is in the repo's
`WEBSOCKET.md`; this is the working reference.

## The five hubs

| Hub | Route | Data |
| --- | --- | --- |
| `orderflow` | `/ws/orderflow` | DEX, GEX, convexity, vanna, charm |
| `classic` | `/ws/classic` | Traditional GEX chain |
| `state_gex` | `/ws/state_gex` | GEX profiles |
| `state_greeks_zero` | `/ws/state_greeks_zero` | Greeks (0DTE): delta, gamma, vanna, charm |
| `state_greeks_one` | `/ws/state_greeks_one` | Greeks (1DTE+) |

## Connection flow

1. `GET /negotiate` with an `Authorization` header → returns the per-hub
   `websocket_urls` and a group **`prefix`**.
2. Connect to a hub URL with the `Sec-WebSocket-Protocol` subprotocol → receive a
   `ConnectedMessage` with a connection id.
3. Send a `JoinGroupMessage` for each group you want (built with the prefix from
   step 1) → receive an `AckMessage`.
4. Receive `DataMessage` broadcasts at the configured interval (`WS_STREAM_INTERVAL`).

## Group naming

All groups follow **`{prefix}_{TICKER}_{hub_type}_{category}`**, where `prefix`
comes from `/negotiate` (default `blue`, set by `WS_GROUP_PREFIX`):

| Hub | Group pattern | Example |
| --- | --- | --- |
| orderflow | `{prefix}_{TICKER}_orderflow_orderflow` | `blue_SPX_orderflow_orderflow` |
| classic | `{prefix}_{TICKER}_classic_{category}` | `blue_SPX_classic_gex_zero` |
| state_gex | `{prefix}_{TICKER}_state_{category}` | `blue_SPX_state_gex_zero` |
| greeks (0DTE) | `{prefix}_{TICKER}_state_{category}` | `blue_SPX_state_gamma_zero` |
| greeks (1DTE+) | `{prefix}_{TICKER}_state_{category}` | `blue_NDX_state_vanna_one` |

The Studio's **Live streams** screen has a group-name builder that assembles these
for you (see [Studio](studio.md)).

## Subprotocols

| Subprotocol | Frame format |
| --- | --- |
| `protobuf.webpubsub.azure.v1` | Binary protobuf (default) |
| `json.reliable.webpubsub.azure.v1` | JSON text (reliable) |
| `json.webpubsub.azure.v1` | JSON text |

Binary frames wrap a Zstd-compressed protobuf payload in a `DataMessage`. Choose a
`json.*` subprotocol if you'd rather parse text frames.

## Keepalive & limits

Server pings every 54s; the client must Pong within 60s. Send buffer 256
messages/client, max frame 512KB, write timeout 10s.
