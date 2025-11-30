# WebSocket Real-Time Feed (Orderflow Hub) Implementation Plan

## Overview

Implement a WebSocket endpoint for the GEX Faker API with **full wire-level compatibility** with the real GexBot API. Starting with the **orderflow hub** only. The Python client (`quant-python-sockets`) should work unchanged except for URL swap.

This enables production-level testing of real-time data consumers against the faker API.

## Current State Analysis

### Existing Infrastructure:
- **Server**: Chi router + oapi-codegen, REST endpoints working
- **Data Loading**: `DataLoader` interface with memory/stream modes (`internal/data/loader.go:14-33`)
- **Playback Tracking**: `IndexCache` tracks per-API-key position (`internal/data/cache.go`)
- **Orderflow Data**: JSONL files at `data/{date}/{ticker}/orderflow/orderflow.jsonl`
- **OrderflowData Model**: Already defined in `internal/data/models.go:38-76`

### Real GexBot WebSocket Protocol:
1. `GET /negotiate` → Returns hub WebSocket URLs
2. Azure Web PubSub protobuf wire protocol
3. Groups: `blue_{ticker}_{package}_{category}` (e.g., `blue_SPX_orderflow_orderflow`)
4. Messages: Zstd-compressed protobuf wrapped in `google.protobuf.Any`

### Key Discoveries:
- Proto schemas exist in `quant-python-sockets/proto/` - can copy directly
- Decompression logic in `decompression_utils.py:146-203` shows exact field mapping
- Integer scaling: `spot*100`, gamma fields `*100`, state/orderflow fields no multiplier
- Python client uses Azure SDK `WebPubSubClient` with `CallbackType.GROUP_MESSAGE`

## Desired End State

After this plan is complete:

1. `GET /negotiate?key=xxx` returns WebSocket URLs matching real API format
2. WebSocket at `/ws/orderflow` accepts connections and speaks Azure Web PubSub protocol
3. Clients can join groups like `blue_SPX_orderflow_orderflow`
4. Server broadcasts Zstd-compressed Orderflow protobuf at configurable intervals
5. Python client (`quant-python-sockets`) connects and receives data with URL change only

### Verification:
```bash
# Start faker
PORT=8080 DATA_DATE=2025-11-28 go run ./cmd/server

# Test negotiate
curl "http://localhost:8080/negotiate?key=test123"
# Returns: {"websocket_urls":{"orderflow":"ws://localhost:8080/ws/orderflow?access_token=..."}}

# Run Python client (with URL pointed to localhost)
cd ../quant-python-sockets
GEXBOT_API_KEY=test123 python main.py
# Should receive orderflow data messages
```

## What We're NOT Doing

- **Other hubs** (classic, state_gex, state_greeks_zero, state_greeks_one) - future expansion
- **JWT authentication** - Simple token passthrough for faker
- **TLS/wss** - HTTP/ws only for local testing
- **Connection limits/rate limiting** - Not needed for faker
- **Persistent sessions/reconnection tokens** - Simplified for faker

## Implementation Approach

**Phasing (Codex-validated)**:
1. Proto definitions + code generation
2. Negotiate endpoint + basic WebSocket upgrade
3. Hub/client connection management
4. Azure Web PubSub protocol implementation
5. Encoder (JSON → Protobuf → Zstd)
6. Streamer (data broadcast loop)

---

## Phase 1: Proto Definitions and Code Generation

### Overview
Copy proto files from Python client, generate Go code, establish proto package structure.

### Changes Required:

#### 1. Create Proto Directory Structure

**Directory**: `proto/`

Copy from `quant-python-sockets/proto/`:
- `orderflow.proto` - Orderflow data schema
- `webpubsub_messages.proto` - Azure Web PubSub protocol

#### 2. Orderflow Proto (already exists, copy as-is)

**File**: `proto/orderflow.proto`

```protobuf
syntax = "proto3";
package orderflow_proto;
option go_package = "github.com/dgnsrekt/gexbot-downloader/internal/proto/orderflow";

message Orderflow {
  int64 timestamp = 1;
  string ticker = 2;
  uint32 spot = 3; // Multiplied by 100
  uint32 zero_major_long_gamma = 4; // *100
  uint32 zero_major_short_gamma = 5; // *100
  uint32 one_major_long_gamma = 6; // *100
  uint32 one_major_short_gamma = 7; // *100
  uint32 zero_major_call_gamma = 8; // *100
  uint32 zero_major_put_gamma = 9; // *100
  uint32 one_major_call_gamma = 10; // *100
  uint32 one_major_put_gamma = 11; // *100
  sint32 zero_convexity_ratio = 12;
  sint32 one_convexity_ratio = 13;
  // ... (all 37 fields from original)
}
```

#### 3. Web PubSub Proto (copy and adapt)

**File**: `proto/webpubsub_messages.proto`

```protobuf
syntax = "proto3";
package azure.webpubsub.v1;
option go_package = "github.com/dgnsrekt/gexbot-downloader/internal/proto/webpubsub";

import "google/protobuf/any.proto";

message UpstreamMessage {
  oneof message {
    JoinGroupMessage join_group_message = 6;
    LeaveGroupMessage leave_group_message = 7;
    PingMessage ping_message = 9;
  }
  message JoinGroupMessage {
    string group = 1;
    optional uint64 ack_id = 2;
  }
  message LeaveGroupMessage {
    string group = 1;
    optional uint64 ack_id = 2;
  }
  message PingMessage {}
}

message DownstreamMessage {
  oneof message {
    AckMessage ack_message = 1;
    DataMessage data_message = 2;
    SystemMessage system_message = 3;
    PongMessage pong_message = 4;
  }
  message AckMessage {
    uint64 ack_id = 1;
    bool success = 2;
  }
  message DataMessage {
    string from = 1;
    optional string group = 2;
    MessageData data = 3;
  }
  message SystemMessage {
    oneof message {
      ConnectedMessage connected_message = 1;
    }
    message ConnectedMessage {
      string connection_id = 1;
      string user_id = 2;
    }
  }
  message PongMessage {}
}

message MessageData {
  oneof data {
    bytes binary_data = 2;
    google.protobuf.Any protobuf_data = 3;
  }
}
```

#### 4. Go Generate Configuration

**File**: `internal/proto/generate.go`

```go
package proto

//go:generate protoc --go_out=. --go_opt=paths=source_relative --proto_path=../../proto ../../proto/orderflow.proto
//go:generate protoc --go_out=. --go_opt=paths=source_relative --proto_path=../../proto ../../proto/webpubsub_messages.proto
```

#### 5. Add Dependencies

**File**: `go.mod` additions

```
github.com/gorilla/websocket v1.5.3
github.com/klauspost/compress v1.17.11
google.golang.org/protobuf v1.35.2
github.com/google/uuid v1.6.0
```

### Success Criteria:

#### Automated Verification:
- [x] Proto files copied to `proto/` directory
- [x] `go generate ./internal/proto/...` succeeds
- [x] Generated files exist: `internal/proto/orderflow/orderflow.pb.go`, `internal/proto/webpubsub/webpubsub_messages.pb.go`
- [x] `go build ./...` succeeds with new dependencies
- [x] `go mod tidy` completes without errors

#### Manual Verification:
- [x] Generated Go types match Python proto definitions
- [x] Import paths resolve correctly in IDE

---

## Phase 2: Negotiate Endpoint

### Overview
Implement `/negotiate` endpoint that returns WebSocket URLs matching real GexBot API format.

### Changes Required:

#### 1. Negotiate Handler

**File**: `internal/websocket/negotiate.go`

```go
package websocket

import (
    "encoding/json"
    "fmt"
    "net/http"

    "github.com/google/uuid"
    "go.uber.org/zap"
)

type NegotiateResponse struct {
    WebsocketURLs map[string]string `json:"websocket_urls"`
}

type NegotiateHandler struct {
    logger *zap.Logger
}

func NewNegotiateHandler(logger *zap.Logger) *NegotiateHandler {
    return &NegotiateHandler{logger: logger}
}

func (h *NegotiateHandler) HandleNegotiate(w http.ResponseWriter, r *http.Request) {
    apiKey := r.URL.Query().Get("key")
    if apiKey == "" {
        http.Error(w, `{"error":"missing key parameter"}`, http.StatusBadRequest)
        return
    }

    // Generate simple access token (apiKey + connection UUID)
    connID := uuid.New().String()
    token := fmt.Sprintf("%s:%s", apiKey, connID)

    // Build WebSocket URL
    scheme := "ws"
    if r.TLS != nil {
        scheme = "wss"
    }
    baseURL := fmt.Sprintf("%s://%s/ws", scheme, r.Host)

    response := NegotiateResponse{
        WebsocketURLs: map[string]string{
            "orderflow": fmt.Sprintf("%s/orderflow?access_token=%s", baseURL, token),
            // Future hubs will be added here
        },
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}
```

#### 2. Router Integration

**File**: `internal/server/server.go` (modify)

Add WebSocket routes outside OpenAPI validation:

```go
// After existing routes, add:
// WebSocket routes (no OpenAPI validation)
r.Get("/negotiate", wsHandler.HandleNegotiate)
r.HandleFunc("/ws/orderflow", wsHub.HandleOrderflowWS)
```

### Success Criteria:

#### Automated Verification:
- [x] `go build ./...` succeeds
- [x] `curl "http://localhost:8080/negotiate?key=test"` returns JSON with `websocket_urls.orderflow`
- [x] Response format matches: `{"websocket_urls":{"orderflow":"ws://..."}}`

#### Manual Verification:
- [x] Token format contains API key and UUID
- [x] URL scheme adapts to HTTP/HTTPS

---

## Phase 3: Hub and Client Connection Management

### Overview
Implement WebSocket hub pattern for managing connections and group subscriptions.

### Changes Required:

#### 1. Hub Manager

**File**: `internal/websocket/hub.go`

```go
package websocket

import (
    "sync"

    "github.com/gorilla/websocket"
    "go.uber.org/zap"
)

type Hub struct {
    name       string
    clients    map[*Client]bool
    groups     map[string]map[*Client]bool // group -> clients
    register   chan *Client
    unregister chan *Client
    broadcast  chan *GroupMessage
    mu         sync.RWMutex
    logger     *zap.Logger
}

type GroupMessage struct {
    Group   string
    Payload []byte
}

func NewHub(name string, logger *zap.Logger) *Hub {
    return &Hub{
        name:       name,
        clients:    make(map[*Client]bool),
        groups:     make(map[string]map[*Client]bool),
        register:   make(chan *Client),
        unregister: make(chan *Client),
        broadcast:  make(chan *GroupMessage, 256),
        logger:     logger,
    }
}

func (h *Hub) Run() {
    for {
        select {
        case client := <-h.register:
            h.mu.Lock()
            h.clients[client] = true
            h.mu.Unlock()
            h.logger.Debug("client registered", zap.String("connID", client.connID))

        case client := <-h.unregister:
            h.mu.Lock()
            if _, ok := h.clients[client]; ok {
                delete(h.clients, client)
                // Remove from all groups
                for group := range client.groups {
                    if clients, ok := h.groups[group]; ok {
                        delete(clients, client)
                    }
                }
                close(client.send)
            }
            h.mu.Unlock()

        case msg := <-h.broadcast:
            h.mu.RLock()
            if clients, ok := h.groups[msg.Group]; ok {
                for client := range clients {
                    select {
                    case client.send <- msg.Payload:
                    default:
                        // Buffer full, disconnect client
                        go func(c *Client) { h.unregister <- c }(client)
                    }
                }
            }
            h.mu.RUnlock()
        }
    }
}

func (h *Hub) JoinGroup(client *Client, group string) {
    h.mu.Lock()
    defer h.mu.Unlock()

    if h.groups[group] == nil {
        h.groups[group] = make(map[*Client]bool)
    }
    h.groups[group][client] = true
    client.groups[group] = true
    h.logger.Debug("client joined group", zap.String("connID", client.connID), zap.String("group", group))
}

func (h *Hub) LeaveGroup(client *Client, group string) {
    h.mu.Lock()
    defer h.mu.Unlock()

    if clients, ok := h.groups[group]; ok {
        delete(clients, client)
    }
    delete(client.groups, group)
}

func (h *Hub) GetActiveGroups() []string {
    h.mu.RLock()
    defer h.mu.RUnlock()

    var groups []string
    for group, clients := range h.groups {
        if len(clients) > 0 {
            groups = append(groups, group)
        }
    }
    return groups
}
```

#### 2. Client Handler

**File**: `internal/websocket/client.go`

```go
package websocket

import (
    "net/http"
    "strings"
    "time"

    "github.com/google/uuid"
    "github.com/gorilla/websocket"
    "go.uber.org/zap"
)

const (
    writeWait      = 10 * time.Second
    pongWait       = 60 * time.Second
    pingPeriod     = (pongWait * 9) / 10
    maxMessageSize = 512 * 1024 // 512KB
    sendBufferSize = 256
)

var upgrader = websocket.Upgrader{
    ReadBufferSize:  1024,
    WriteBufferSize: 1024,
    CheckOrigin:     func(r *http.Request) bool { return true },
}

type Client struct {
    hub    *Hub
    conn   *websocket.Conn
    send   chan []byte
    apiKey string
    connID string
    groups map[string]bool
}

func (h *Hub) HandleOrderflowWS(w http.ResponseWriter, r *http.Request) {
    // Extract token
    token := r.URL.Query().Get("access_token")
    if token == "" {
        http.Error(w, "missing access_token", http.StatusUnauthorized)
        return
    }

    // Parse token (format: apiKey:connID)
    parts := strings.SplitN(token, ":", 2)
    apiKey := parts[0]
    connID := uuid.New().String()

    // Upgrade connection
    conn, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
        h.logger.Error("websocket upgrade failed", zap.Error(err))
        return
    }

    client := &Client{
        hub:    h,
        conn:   conn,
        send:   make(chan []byte, sendBufferSize),
        apiKey: apiKey,
        connID: connID,
        groups: make(map[string]bool),
    }

    h.register <- client

    // Send ConnectedMessage
    connectedMsg := buildConnectedMessage(connID, apiKey)
    client.send <- connectedMsg

    go client.writePump()
    go client.readPump()
}

func (c *Client) readPump() {
    defer func() {
        c.hub.unregister <- c
        c.conn.Close()
    }()

    c.conn.SetReadLimit(maxMessageSize)
    c.conn.SetReadDeadline(time.Now().Add(pongWait))
    c.conn.SetPongHandler(func(string) error {
        c.conn.SetReadDeadline(time.Now().Add(pongWait))
        return nil
    })

    for {
        _, message, err := c.conn.ReadMessage()
        if err != nil {
            break
        }
        c.handleMessage(message)
    }
}

func (c *Client) writePump() {
    ticker := time.NewTicker(pingPeriod)
    defer func() {
        ticker.Stop()
        c.conn.Close()
    }()

    for {
        select {
        case message, ok := <-c.send:
            c.conn.SetWriteDeadline(time.Now().Add(writeWait))
            if !ok {
                c.conn.WriteMessage(websocket.CloseMessage, []byte{})
                return
            }
            if err := c.conn.WriteMessage(websocket.BinaryMessage, message); err != nil {
                return
            }

        case <-ticker.C:
            c.conn.SetWriteDeadline(time.Now().Add(writeWait))
            if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
                return
            }
        }
    }
}
```

### Success Criteria:

#### Automated Verification:
- [x] `go build ./...` succeeds
- [x] Hub starts and processes register/unregister channels
- [x] WebSocket upgrade succeeds at `/ws/orderflow`

#### Manual Verification:
- [x] `wscat -c "ws://localhost:8080/ws/orderflow?access_token=test:123"` connects
- [x] ConnectedMessage received on connection

---

## Phase 4: Azure Web PubSub Protocol

### Overview
Implement protobuf message building for Azure Web PubSub protocol.

### Changes Required:

#### 1. Protocol Implementation

**File**: `internal/websocket/protocol.go`

```go
package websocket

import (
    "google.golang.org/protobuf/proto"
    "google.golang.org/protobuf/types/known/anypb"

    pb "github.com/dgnsrekt/gexbot-downloader/internal/proto/webpubsub"
)

func buildConnectedMessage(connectionID, userID string) []byte {
    msg := &pb.DownstreamMessage{
        Message: &pb.DownstreamMessage_SystemMessage_{
            SystemMessage: &pb.DownstreamMessage_SystemMessage{
                Message: &pb.DownstreamMessage_SystemMessage_ConnectedMessage_{
                    ConnectedMessage: &pb.DownstreamMessage_SystemMessage_ConnectedMessage{
                        ConnectionId: connectionID,
                        UserId:       userID,
                    },
                },
            },
        },
    }
    data, _ := proto.Marshal(msg)
    return data
}

func buildAckMessage(ackID uint64, success bool) []byte {
    msg := &pb.DownstreamMessage{
        Message: &pb.DownstreamMessage_AckMessage_{
            AckMessage: &pb.DownstreamMessage_AckMessage{
                AckId:   ackID,
                Success: success,
            },
        },
    }
    data, _ := proto.Marshal(msg)
    return data
}

func buildDataMessage(group string, compressedData []byte) []byte {
    anyMsg := &anypb.Any{
        TypeUrl: "proto.orderflow",
        Value:   compressedData,
    }

    msg := &pb.DownstreamMessage{
        Message: &pb.DownstreamMessage_DataMessage_{
            DataMessage: &pb.DownstreamMessage_DataMessage{
                From:  "server",
                Group: &group,
                Data: &pb.MessageData{
                    Data: &pb.MessageData_ProtobufData{
                        ProtobufData: anyMsg,
                    },
                },
            },
        },
    }
    data, _ := proto.Marshal(msg)
    return data
}

func buildPongMessage() []byte {
    msg := &pb.DownstreamMessage{
        Message: &pb.DownstreamMessage_PongMessage_{
            PongMessage: &pb.DownstreamMessage_PongMessage{},
        },
    }
    data, _ := proto.Marshal(msg)
    return data
}
```

#### 2. Message Handler (in client.go)

```go
func (c *Client) handleMessage(data []byte) {
    var msg pb.UpstreamMessage
    if err := proto.Unmarshal(data, &msg); err != nil {
        c.hub.logger.Warn("failed to parse upstream message", zap.Error(err))
        return
    }

    switch m := msg.Message.(type) {
    case *pb.UpstreamMessage_JoinGroupMessage_:
        group := m.JoinGroupMessage.Group
        // Validate group format: blue_{ticker}_orderflow_orderflow
        if isValidOrderflowGroup(group) {
            c.hub.JoinGroup(c, group)
            if m.JoinGroupMessage.AckId != nil {
                c.send <- buildAckMessage(*m.JoinGroupMessage.AckId, true)
            }
        }

    case *pb.UpstreamMessage_LeaveGroupMessage_:
        group := m.LeaveGroupMessage.Group
        c.hub.LeaveGroup(c, group)
        if m.LeaveGroupMessage.AckId != nil {
            c.send <- buildAckMessage(*m.LeaveGroupMessage.AckId, true)
        }

    case *pb.UpstreamMessage_PingMessage_:
        c.send <- buildPongMessage()
    }
}

func isValidOrderflowGroup(group string) bool {
    // Format: blue_{ticker}_orderflow_orderflow
    return strings.HasPrefix(group, "blue_") && strings.HasSuffix(group, "_orderflow_orderflow")
}
```

### Success Criteria:

#### Automated Verification:
- [x] `go build ./...` succeeds
- [x] Unit test: `buildConnectedMessage` produces valid protobuf
- [x] Unit test: `buildDataMessage` wraps data in correct Any format

#### Manual Verification:
- [x] Python client can parse ConnectedMessage
- [x] JoinGroupMessage/AckMessage round-trip works

---

## Phase 5: Encoder (JSON → Protobuf → Zstd)

### Overview
Convert JSON orderflow data to wire format matching real API.

### Changes Required:

#### 1. Encoder Implementation

**File**: `internal/websocket/encoder.go`

```go
package websocket

import (
    "encoding/json"

    "github.com/klauspost/compress/zstd"

    "github.com/dgnsrekt/gexbot-downloader/internal/data"
    ofpb "github.com/dgnsrekt/gexbot-downloader/internal/proto/orderflow"
    "google.golang.org/protobuf/proto"
)

type Encoder struct {
    zstdEncoder *zstd.Encoder
}

func NewEncoder() (*Encoder, error) {
    enc, err := zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.SpeedDefault))
    if err != nil {
        return nil, err
    }
    return &Encoder{zstdEncoder: enc}, nil
}

func (e *Encoder) EncodeOrderflow(jsonData []byte) ([]byte, error) {
    // 1. Parse JSON into OrderflowData
    var of data.OrderflowData
    if err := json.Unmarshal(jsonData, &of); err != nil {
        return nil, err
    }

    // 2. Convert to protobuf with integer scaling
    pbMsg := &ofpb.Orderflow{
        Timestamp:             of.Timestamp,
        Ticker:                of.Ticker,
        Spot:                  uint32(of.Spot * 100),
        ZeroMajorLongGamma:    uint32(of.ZMlgamma * 100),
        ZeroMajorShortGamma:   uint32(of.ZMsgamma * 100),
        OneMajorLongGamma:     uint32(of.OMlgamma * 100),
        OneMajorShortGamma:    uint32(of.OMsgamma * 100),
        ZeroMajorCallGamma:    uint32(of.ZeroMcall * 100),
        ZeroMajorPutGamma:     uint32(of.ZeroMput * 100),
        OneMajorCallGamma:     uint32(of.OneMcall * 100),
        OneMajorPutGamma:      uint32(of.OneMput * 100),
        // State fields (no multiplier)
        ZeroConvexityRatio:    int32(of.Zcvr),
        OneConvexityRatio:     int32(of.Ocvr),
        ZeroGexRatio:          int32(of.Zgr),
        OneGexRatio:           int32(of.Ogr),
        ZeroNetVanna:          int32(of.Zvanna),
        OneNetVanna:           int32(of.Ovanna),
        ZeroNetCharm:          int32(of.Zcharm),
        OneNetCharm:           int32(of.Ocharm),
        ZeroAggTotalDex:       int32(of.AggDex),
        OneAggTotalDex:        int32(of.OneAggDex),
        ZeroAggCallDex:        int32(of.AggCallDex),
        OneAggCallDex:         int32(of.OneAggCallDex),
        ZeroAggPutDex:         int32(of.AggPutDex),
        OneAggPutDex:          int32(of.OneAggPutDex),
        ZeroNetTotalDex:       int32(of.NetDex),
        OneNetTotalDex:        int32(of.OneNetDex),
        ZeroNetCallDex:        int32(of.NetCallDex),
        OneNetCallDex:         int32(of.OneNetCallDex),
        ZeroNetPutDex:         int32(of.NetPutDex),
        OneNetPutDex:          int32(of.OneNetPutDex),
        // Orderflow fields (no multiplier)
        DexOrderflow:          int32(of.Dexoflow),
        GexOrderflow:          int32(of.Gexoflow),
        ConvexityOrderflow:    int32(of.Cvroflow),
        OneDexOrderflow:       int32(of.OneDexoflow),
        OneGexOrderflow:       int32(of.OneGexoflow),
        OneConvexityOrderflow: int32(of.OneCvroflow),
    }

    // 3. Serialize to protobuf
    pbData, err := proto.Marshal(pbMsg)
    if err != nil {
        return nil, err
    }

    // 4. Compress with zstd
    compressed := e.zstdEncoder.EncodeAll(pbData, nil)

    return compressed, nil
}

func (e *Encoder) Close() {
    e.zstdEncoder.Close()
}
```

### Success Criteria:

#### Automated Verification:
- [x] `go build ./...` succeeds
- [x] Unit test: Encode sample JSON, decompress with Python, verify fields match
- [x] Fixture test: Compare encoded bytes against known-good capture

#### Manual Verification:
- [x] Python `decompress_orderflow_message()` successfully decodes faker output

---

## Phase 6: Streamer (Data Broadcast)

### Overview
Stream data from JSONL files to subscribed clients at configurable intervals.

### Changes Required:

#### 1. Streamer Implementation

**File**: `internal/websocket/streamer.go`

```go
package websocket

import (
    "context"
    "strings"
    "sync"
    "time"

    "go.uber.org/zap"

    "github.com/dgnsrekt/gexbot-downloader/internal/data"
)

type Streamer struct {
    hub      *Hub
    loader   data.DataLoader
    encoder  *Encoder
    interval time.Duration
    indexes  map[string]int // ticker -> current index
    mu       sync.RWMutex
    stop     chan struct{}
    logger   *zap.Logger
}

func NewStreamer(hub *Hub, loader data.DataLoader, interval time.Duration, logger *zap.Logger) (*Streamer, error) {
    enc, err := NewEncoder()
    if err != nil {
        return nil, err
    }

    return &Streamer{
        hub:      hub,
        loader:   loader,
        encoder:  enc,
        interval: interval,
        indexes:  make(map[string]int),
        stop:     make(chan struct{}),
        logger:   logger,
    }, nil
}

func (s *Streamer) Run() {
    ticker := time.NewTicker(s.interval)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            s.broadcastNext()
        case <-s.stop:
            return
        }
    }
}

func (s *Streamer) Stop() {
    close(s.stop)
    s.encoder.Close()
}

func (s *Streamer) broadcastNext() {
    groups := s.hub.GetActiveGroups()
    if len(groups) == 0 {
        return
    }

    ctx := context.Background()

    for _, group := range groups {
        // Parse group: blue_{ticker}_orderflow_orderflow
        ticker := extractTicker(group)
        if ticker == "" {
            continue
        }

        // Get current index for this ticker
        s.mu.Lock()
        idx := s.indexes[ticker]
        s.mu.Unlock()

        // Get data length
        length, err := s.loader.GetLength(ticker, "orderflow", "orderflow")
        if err != nil {
            s.logger.Warn("failed to get length", zap.String("ticker", ticker), zap.Error(err))
            continue
        }

        // Check if exhausted (wrap around for continuous playback)
        if idx >= length {
            idx = 0
        }

        // Get raw JSON data
        rawJSON, err := s.loader.GetRawAtIndex(ctx, ticker, "orderflow", "orderflow", idx)
        if err != nil {
            s.logger.Warn("failed to get data", zap.String("ticker", ticker), zap.Int("index", idx), zap.Error(err))
            continue
        }

        // Encode to protobuf + zstd
        encoded, err := s.encoder.EncodeOrderflow(rawJSON)
        if err != nil {
            s.logger.Warn("failed to encode", zap.String("ticker", ticker), zap.Error(err))
            continue
        }

        // Build DataMessage and broadcast
        msg := buildDataMessage(group, encoded)
        s.hub.broadcast <- &GroupMessage{Group: group, Payload: msg}

        // Advance index
        s.mu.Lock()
        s.indexes[ticker] = idx + 1
        s.mu.Unlock()
    }
}

func extractTicker(group string) string {
    // Format: blue_{ticker}_orderflow_orderflow
    if !strings.HasPrefix(group, "blue_") {
        return ""
    }
    trimmed := strings.TrimPrefix(group, "blue_")
    // Find _orderflow_orderflow suffix
    idx := strings.Index(trimmed, "_orderflow_orderflow")
    if idx < 0 {
        return ""
    }
    return trimmed[:idx]
}
```

#### 2. Server Integration

**File**: `cmd/server/main.go` (additions)

```go
// After creating server, add WebSocket components:

// Create WebSocket hub
wsHub := websocket.NewHub("orderflow", logger)
go wsHub.Run()

// Create and start streamer
streamer, err := websocket.NewStreamer(wsHub, loader, time.Second, logger)
if err != nil {
    logger.Error("failed to create streamer", zap.Error(err))
    return 1
}
go streamer.Run()
defer streamer.Stop()

// Create negotiate handler
negotiateHandler := websocket.NewNegotiateHandler(logger)

// Update router creation to pass websocket components
router, err := server.NewRouter(srv, wsHub, negotiateHandler, logger)
```

#### 3. Configuration Updates

**File**: `internal/config/server.go` (additions)

```go
type ServerConfig struct {
    // ... existing fields ...
    WSEnabled        bool          `envconfig:"WS_ENABLED" default:"true"`
    WSStreamInterval time.Duration `envconfig:"WS_STREAM_INTERVAL" default:"1s"`
}
```

### Success Criteria:

#### Automated Verification:
- [x] `go build ./...` succeeds
- [x] Server starts with WebSocket components
- [x] `WS_STREAM_INTERVAL` configurable via environment

#### Manual Verification:
- [x] Python client receives orderflow data messages
- [x] Data advances each interval
- [x] Multiple tickers stream independently

---

## Phase 7: Integration Testing

### Overview
End-to-end validation with Python client.

### Test Script:

```bash
#!/bin/bash
# test_websocket.sh

# 1. Start faker server
PORT=8080 DATA_DATE=2025-11-28 WS_STREAM_INTERVAL=500ms go run ./cmd/server &
SERVER_PID=$!
sleep 2

# 2. Test negotiate endpoint
echo "Testing negotiate..."
curl -s "http://localhost:8080/negotiate?key=test123" | jq .

# 3. Run Python client (requires manual URL update in main.py)
echo "Starting Python client..."
cd ../quant-python-sockets
# Update BASE_URL to http://localhost:8080
# Uncomment "orderflow" in ACTIVE_ORDERFLOW_CATEGORIES
# Run: python main.py

# 4. Cleanup
kill $SERVER_PID
```

### Success Criteria:

#### Automated Verification:
- [x] `go test ./internal/websocket/...` passes
- [x] Encoder fixture tests match expected wire format

#### Manual Verification:
- [x] Python client connects without code changes (URL only)
- [x] `decompress_orderflow_message()` returns valid data
- [x] Continuous data stream for subscribed tickers

---

## Performance Considerations

- **Pre-compression**: Compress once per tick, reuse payload for all subscribers of same group
- **Bounded buffers**: 256-message send buffer per client; overflow disconnects client
- **Write deadlines**: 10-second timeout prevents blocking on slow clients
- **Encoder pooling**: Consider `sync.Pool` for encoders if high connection count

## Migration Notes

No migration required - this is a new feature addition.

## References

- Python client: `../quant-python-sockets/main.py:168-280`
- Decompression logic: `../quant-python-sockets/decompression_utils.py:146-203`
- Proto schemas: `../quant-python-sockets/proto/`
- Existing server: `internal/server/server.go`
- Data loader: `internal/data/loader.go:14-33`

## Codex Mentorship Notes

Key guidance from Codex validation:
- Add `github.com/google/uuid` for connection IDs
- Consider encoder pool for zstd to avoid reinit cost
- Add fixture-based encoder tests before wiring network
- Validate group names, cap groups per connection
- Use write deadlines and bounded buffers for backpressure
- Pre-compress per-tick for efficiency
