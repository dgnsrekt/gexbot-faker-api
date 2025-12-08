# WebSocket Streaming Protocol

Real-time market data streaming via Azure Web PubSub compatible WebSocket protocol with Protobuf + Zstd compression.

## Connection Flow

```
1. GET /negotiate (Authorization: Basic <API_KEY>)
   └─> Returns WebSocket URLs and group prefix

2. Connect to hub URL with subprotocol header
   └─> Receive ConnectedMessage with connectionId

3. Send JoinGroupMessage for desired subscriptions (use prefix from step 1)
   └─> Receive AckMessage confirming subscription

4. Receive DataMessage broadcasts at configured interval
```

## Hubs

| Hub               | Route                   | Data Type         | Description                       |
| ----------------- | ----------------------- | ----------------- | --------------------------------- |
| orderflow         | `/ws/orderflow`         | Orderflow metrics | DEX, GEX, convexity, vanna, charm |
| classic           | `/ws/classic`           | Classic GEX       | Traditional GEX chain data        |
| state_gex         | `/ws/state_gex`         | State GEX         | Orderflow-based GEX profiles      |
| state_greeks_zero | `/ws/state_greeks_zero` | Greeks (0DTE)     | Delta, gamma, vanna, charm        |
| state_greeks_one  | `/ws/state_greeks_one`  | Greeks (1DTE+)    | Delta, gamma, vanna, charm        |

## Group Naming

All group names follow the pattern: `{prefix}_{TICKER}_{hub_type}_{category}`

The `prefix` is returned by the `/negotiate` endpoint. Use it when constructing group names.

### Orderflow Hub

```
{prefix}_{TICKER}_orderflow_orderflow
```

Example: `blue_SPX_orderflow_orderflow`

### Classic Hub

```
{prefix}_{TICKER}_classic_{category}
```

Categories: `gex_full`, `gex_zero`, `gex_one`

Examples:

- `blue_SPX_classic_gex_zero`
- `blue_ES_SPX_classic_gex_full`

### State GEX Hub

```
{prefix}_{TICKER}_state_{category}
```

Categories: `gex_full`, `gex_zero`, `gex_one`

Examples:

- `blue_SPX_state_gex_zero`
- `blue_NDX_state_gex_one`

### State Greeks Zero Hub

```
{prefix}_{TICKER}_state_{category}
```

Categories: `delta_zero`, `gamma_zero`, `vanna_zero`, `charm_zero`

Examples:

- `blue_SPX_state_delta_zero`
- `blue_ES_SPX_state_gamma_zero`

### State Greeks One Hub

```
{prefix}_{TICKER}_state_{category}
```

Categories: `delta_one`, `gamma_one`, `vanna_one`, `charm_one`

Examples:

- `blue_SPX_state_delta_one`
- `blue_NDX_state_vanna_one`

## Protocol Negotiation

Set the `Sec-WebSocket-Protocol` header during connection:

| Subprotocol                        | Format          | Message Type    |
| ---------------------------------- | --------------- | --------------- |
| `protobuf.webpubsub.azure.v1`      | Binary protobuf | `BinaryMessage` |
| `json.reliable.webpubsub.azure.v1` | JSON text       | `TextMessage`   |
| `json.webpubsub.azure.v1`          | JSON text       | `TextMessage`   |

Default: Protobuf if no preference specified.

## Message Types

### Upstream (Client → Server)

**JoinGroupMessage**

```protobuf
message JoinGroupMessage {
  string group = 1;           // Group name to join
  optional uint64 ack_id = 2; // Optional acknowledgment ID
}
```

**LeaveGroupMessage**

```protobuf
message LeaveGroupMessage {
  string group = 1;
  optional uint64 ack_id = 2;
}
```

**PingMessage**

```protobuf
message PingMessage {}
```

### Downstream (Server → Client)

**ConnectedMessage** (sent on connection)

```protobuf
message ConnectedMessage {
  string connection_id = 1;
  string user_id = 2;
}
```

**AckMessage** (response to join/leave)

```protobuf
message AckMessage {
  uint64 ack_id = 1;
  bool success = 2;
}
```

**DataMessage** (broadcast data)

```protobuf
message DataMessage {
  string from = 1;              // "server"
  optional string group = 2;    // Group name
  MessageData data = 3;         // Payload
}

message MessageData {
  oneof data {
    string text_data = 1;
    bytes binary_data = 2;
    google.protobuf.Any protobuf_data = 3;
  }
}
```

**PongMessage**

```protobuf
message PongMessage {}
```

## Data Encoding

Data flows through this encoding pipeline:

```
JSON (from JSONL file)
    ↓
Parse to Go struct
    ↓
Scale floats to integers (×100 or ×1000)
    ↓
Marshal to Protobuf
    ↓
Zstd compression
    ↓
Wrap in DataMessage
    ↓
Wire format (Binary or JSON+Base64)
```

## Protobuf Definitions

Located in `proto/` directory:

### orderflow.proto

Orderflow metrics with 37 fields including:

- Gamma levels (major long/short)
- State ratios (convexity, GEX, vanna, charm)
- DEX metrics (aggregate and net)
- Orderflow indicators

### gex.proto

GEX chain data with:

- Spot price, zero gamma level
- Major positive/negative levels
- Strikes array with priors
- Max priors (6 lookback periods)

### option_profile.proto

Greek profile data with:

- Spot, major call/put gamma
- Major long/short gamma
- Mini contracts array with IV and volume

### webpubsub_messages.proto

Azure Web PubSub protocol messages for upstream/downstream communication.

## Example: Python Client

```python
import websocket
import struct

# 1. Negotiate
resp = requests.get(
    "http://localhost:8080/negotiate",
    headers={"Authorization": "Basic myapikey"}
)
data = resp.json()
urls = data["websocket_urls"]
prefix = data["prefix"]

# 2. Connect with protobuf subprotocol
ws = websocket.create_connection(
    urls["state_gex"],
    subprotocols=["protobuf.webpubsub.azure.v1"]
)

# 3. Receive ConnectedMessage
connected_msg = ws.recv()

# 4. Join group (protobuf format) - use prefix from negotiate
group = f"{prefix}_SPX_state_gex_zero"
join_msg = build_join_group_message(group, ack_id=1)
ws.send(join_msg, opcode=websocket.ABNF.OPCODE_BINARY)

# 5. Receive ACK
ack = ws.recv()

# 6. Receive data broadcasts
while True:
    data = ws.recv()
    # Unwrap DataMessage → decompress zstd → parse protobuf
```

## Keepalive

- Server sends Ping every 54 seconds
- Client must respond with Pong within 60 seconds
- Failure to respond triggers connection close

## Buffer Limits

- Send buffer: 256 messages per client
- Max message size: 512KB
- Write timeout: 10 seconds
