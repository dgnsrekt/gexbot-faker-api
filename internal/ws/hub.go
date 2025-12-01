package ws

import (
	"context"
	"sync"

	"go.uber.org/zap"
)

// GroupValidator is a function that validates group names for a hub.
type GroupValidator func(group string) bool

// Hub manages WebSocket connections and group subscriptions.
type Hub struct {
	name           string
	clients        map[*Client]bool
	groups         map[string]map[*Client]bool // group -> clients
	register       chan *Client
	unregister     chan *Client
	broadcast      chan *GroupMessage
	mu             sync.RWMutex
	logger         *zap.Logger
	groupValidator GroupValidator
}

// GroupMessage represents a message to broadcast to a group.
type GroupMessage struct {
	Group   string
	Payload []byte
}

// NewHub creates a new Hub with a group validator.
func NewHub(name string, logger *zap.Logger, validator GroupValidator) *Hub {
	return &Hub{
		name:           name,
		clients:        make(map[*Client]bool),
		groups:         make(map[string]map[*Client]bool),
		register:       make(chan *Client),
		unregister:     make(chan *Client),
		broadcast:      make(chan *GroupMessage, 256),
		logger:         logger,
		groupValidator: validator,
	}
}

// ValidateGroup checks if a group name is valid for this hub.
func (h *Hub) ValidateGroup(group string) bool {
	if h.groupValidator == nil {
		return true // No validator means all groups are valid
	}
	return h.groupValidator(group)
}

// Run processes hub events. Call this in a goroutine.
// Returns when context is cancelled.
func (h *Hub) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			h.logger.Info("hub shutting down", zap.String("hub", h.name))
			h.shutdown()
			return

		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			h.logger.Debug("client registered",
				zap.String("hub", h.name),
				zap.String("connID", client.connID),
			)

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				// Remove from all groups
				for group := range client.groups {
					if clients, ok := h.groups[group]; ok {
						delete(clients, client)
						if len(clients) == 0 {
							delete(h.groups, group)
						}
					}
				}
				close(client.send)
			}
			h.mu.Unlock()
			h.logger.Debug("client unregistered",
				zap.String("hub", h.name),
				zap.String("connID", client.connID),
			)

		case msg := <-h.broadcast:
			h.mu.RLock()
			if clients, ok := h.groups[msg.Group]; ok {
				for client := range clients {
					select {
					case client.send <- msg.Payload:
					default:
						// Buffer full, schedule disconnect
						go func(c *Client) {
							h.unregister <- c
						}(client)
					}
				}
			}
			h.mu.RUnlock()
		}
	}
}

// shutdown gracefully closes all client connections.
func (h *Hub) shutdown() {
	h.mu.Lock()
	defer h.mu.Unlock()

	for client := range h.clients {
		close(client.send)
		delete(h.clients, client)
	}
	h.groups = make(map[string]map[*Client]bool)
}

// JoinGroup adds a client to a group.
func (h *Hub) JoinGroup(client *Client, group string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.groups[group] == nil {
		h.groups[group] = make(map[*Client]bool)
	}
	h.groups[group][client] = true
	client.groups[group] = true

	h.logger.Debug("client joined group",
		zap.String("hub", h.name),
		zap.String("connID", client.connID),
		zap.String("group", group),
	)
}

// LeaveGroup removes a client from a group.
func (h *Hub) LeaveGroup(client *Client, group string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if clients, ok := h.groups[group]; ok {
		delete(clients, client)
		if len(clients) == 0 {
			delete(h.groups, group)
		}
	}
	delete(client.groups, group)

	h.logger.Debug("client left group",
		zap.String("hub", h.name),
		zap.String("connID", client.connID),
		zap.String("group", group),
	)
}

// GetActiveGroups returns all groups with at least one subscriber.
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

// Broadcast sends a message to all clients in a group.
func (h *Hub) Broadcast(group string, payload []byte) {
	h.broadcast <- &GroupMessage{Group: group, Payload: payload}
}

// BroadcastData sends encoded data to all clients in a group.
// Each client formats the data message according to its negotiated protocol.
// typeUrl should be "proto.orderflow", "proto.gex", "proto.greek", etc.
func (h *Hub) BroadcastData(group string, encodedData []byte, typeUrl string) {
	h.mu.RLock()
	clients, ok := h.groups[group]
	if !ok {
		h.mu.RUnlock()
		return
	}
	// Copy clients to avoid holding lock during send
	clientList := make([]*Client, 0, len(clients))
	for client := range clients {
		clientList = append(clientList, client)
	}
	h.mu.RUnlock()

	for _, client := range clientList {
		// Build message in client's protocol format
		msg := client.buildDataMsg(group, encodedData, typeUrl)
		select {
		case client.send <- msg:
		default:
			// Buffer full, schedule disconnect
			go func(c *Client) {
				h.unregister <- c
			}(client)
		}
	}
}

// BroadcastDataDual sends data to all clients in a group with format-aware routing.
// Protobuf clients receive encodedData (Zstd-compressed protobuf).
// JSON clients receive rawJSON (original JSON format with arrays intact).
// This ensures JSON clients get data matching the real GexBot API wire format.
func (h *Hub) BroadcastDataDual(group string, encodedData []byte, rawJSON []byte, typeUrl string) {
	h.mu.RLock()
	clients, ok := h.groups[group]
	if !ok {
		h.mu.RUnlock()
		return
	}
	// Copy clients to avoid holding lock during send
	clientList := make([]*Client, 0, len(clients))
	for client := range clients {
		clientList = append(clientList, client)
	}
	h.mu.RUnlock()

	for _, client := range clientList {
		var msg []byte
		if client.protocol == "json" && rawJSON != nil {
			// JSON clients get raw JSON format (arrays preserved)
			msg = buildDataMessageJSONRaw(group, rawJSON)
		} else if client.protocol == "json" {
			// Fallback to base64-encoded protobuf for JSON clients without rawJSON
			msg = buildDataMessageJSON(group, encodedData, typeUrl)
		} else {
			// Protobuf clients get binary format
			msg = buildDataMessage(group, encodedData, typeUrl)
		}
		select {
		case client.send <- msg:
		default:
			// Buffer full, schedule disconnect
			go func(c *Client) {
				h.unregister <- c
			}(client)
		}
	}
}
