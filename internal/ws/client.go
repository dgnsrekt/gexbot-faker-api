package ws

import (
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 512 * 1024 // 512KB

	// Send buffer size per client.
	sendBufferSize = 256
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true }, // Allow all origins for faker
	Subprotocols: []string{
		"protobuf.webpubsub.azure.v1",
		"json.reliable.webpubsub.azure.v1",
		"json.webpubsub.azure.v1",
	},
}

// Client represents a WebSocket client connection.
type Client struct {
	hub      *Hub
	conn     *websocket.Conn
	send     chan []byte
	apiKey   string
	connID   string
	groups   map[string]bool
	logger   *zap.Logger
	protocol string // "protobuf" or "json"
}

// HandleOrderflowWS handles WebSocket upgrade for the orderflow hub.
func (h *Hub) HandleOrderflowWS(w http.ResponseWriter, r *http.Request) {
	// Extract access token
	token := r.URL.Query().Get("access_token")
	if token == "" {
		http.Error(w, "missing access_token", http.StatusUnauthorized)
		return
	}

	// Parse token (format: apiKey:originalConnID)
	parts := strings.SplitN(token, ":", 2)
	apiKey := parts[0]
	connID := uuid.New().String() // Generate new connID for this connection

	// Negotiate subprotocol - check what client requested
	protocol := "protobuf" // default
	var responseHeader http.Header
	for _, proto := range websocket.Subprotocols(r) {
		switch proto {
		case "protobuf.webpubsub.azure.v1":
			protocol = "protobuf"
			responseHeader = http.Header{"Sec-WebSocket-Protocol": {proto}}
		case "json.reliable.webpubsub.azure.v1", "json.webpubsub.azure.v1":
			protocol = "json"
			responseHeader = http.Header{"Sec-WebSocket-Protocol": {proto}}
		}
		if responseHeader != nil {
			break
		}
	}

	h.logger.Debug("websocket subprotocol negotiated",
		zap.String("protocol", protocol),
		zap.Strings("requested", websocket.Subprotocols(r)),
	)

	// Upgrade to WebSocket
	conn, err := upgrader.Upgrade(w, r, responseHeader)
	if err != nil {
		h.logger.Error("websocket upgrade failed", zap.Error(err))
		return
	}

	client := &Client{
		hub:      h,
		conn:     conn,
		send:     make(chan []byte, sendBufferSize),
		apiKey:   apiKey,
		connID:   connID,
		groups:   make(map[string]bool),
		logger:   h.logger,
		protocol: protocol,
	}

	h.register <- client

	// Send ConnectedMessage per negotiated protocol
	var connectedMsg []byte
	if protocol == "json" {
		connectedMsg = buildConnectedMessageJSON(connID, apiKey)
	} else {
		connectedMsg = buildConnectedMessage(connID, apiKey)
	}
	client.send <- connectedMsg

	// Start read/write pumps
	go client.writePump()
	go client.readPump()
}

// readPump reads messages from the WebSocket connection.
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
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.logger.Debug("websocket read error",
					zap.String("connID", c.connID),
					zap.Error(err),
				)
			}
			break
		}
		c.handleMessage(message)
	}
}

// writePump writes messages to the WebSocket connection.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	// Determine message type based on protocol
	msgType := websocket.BinaryMessage
	if c.protocol == "json" {
		msgType = websocket.TextMessage
	}

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// Channel closed, send close message
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(msgType, message); err != nil {
				c.logger.Debug("websocket write error",
					zap.String("connID", c.connID),
					zap.Error(err),
				)
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

// handleMessage processes an incoming upstream message.
func (c *Client) handleMessage(data []byte) {
	// Parse based on protocol
	var msg any
	var err error
	if c.protocol == "json" {
		msg, err = parseUpstreamMessageJSON(data)
	} else {
		msg, err = parseUpstreamMessage(data)
	}

	if err != nil {
		c.logger.Debug("failed to parse upstream message",
			zap.String("connID", c.connID),
			zap.String("protocol", c.protocol),
			zap.Error(err),
		)
		return
	}

	switch m := msg.(type) {
	case *joinGroupRequest:
		if isValidOrderflowGroup(m.group) {
			c.hub.JoinGroup(c, m.group)
			if m.ackID != nil {
				c.send <- c.buildAck(*m.ackID, true)
			}
		} else {
			c.logger.Debug("invalid group name",
				zap.String("connID", c.connID),
				zap.String("group", m.group),
			)
			if m.ackID != nil {
				c.send <- c.buildAck(*m.ackID, false)
			}
		}

	case *leaveGroupRequest:
		c.hub.LeaveGroup(c, m.group)
		if m.ackID != nil {
			c.send <- c.buildAck(*m.ackID, true)
		}

	case *pingRequest:
		c.send <- c.buildPong()
	}
}

// buildAck creates an ack message in the correct format for this client's protocol.
func (c *Client) buildAck(ackID uint64, success bool) []byte {
	if c.protocol == "json" {
		return buildAckMessageJSON(ackID, success)
	}
	return buildAckMessage(ackID, success)
}

// buildPong creates a pong message in the correct format for this client's protocol.
func (c *Client) buildPong() []byte {
	if c.protocol == "json" {
		return buildPongMessageJSON()
	}
	return buildPongMessage()
}

// buildDataMsg creates a data message in the correct format for this client's protocol.
func (c *Client) buildDataMsg(group string, encodedData []byte) []byte {
	if c.protocol == "json" {
		return buildDataMessageJSON(group, encodedData)
	}
	return buildDataMessage(group, encodedData)
}

// isValidOrderflowGroup validates the orderflow group name format.
// Expected format: blue_{ticker}_orderflow_orderflow
func isValidOrderflowGroup(group string) bool {
	return strings.HasPrefix(group, "blue_") && strings.HasSuffix(group, "_orderflow_orderflow")
}
