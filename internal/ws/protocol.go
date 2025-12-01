package ws

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	pb "github.com/dgnsrekt/gexbot-downloader/internal/ws/generated/webpubsub"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

// Upstream message types for internal routing
type (
	joinGroupRequest struct {
		group string
		ackID *uint64
	}
	leaveGroupRequest struct {
		group string
		ackID *uint64
	}
	pingRequest struct{}
)

// parseUpstreamMessage parses a protobuf-encoded UpstreamMessage.
func parseUpstreamMessage(data []byte) (any, error) {
	var msg pb.UpstreamMessage
	if err := proto.Unmarshal(data, &msg); err != nil {
		return nil, fmt.Errorf("unmarshal upstream message: %w", err)
	}

	switch m := msg.Message.(type) {
	case *pb.UpstreamMessage_JoinGroupMessage_:
		return &joinGroupRequest{
			group: m.JoinGroupMessage.Group,
			ackID: m.JoinGroupMessage.AckId,
		}, nil

	case *pb.UpstreamMessage_LeaveGroupMessage_:
		return &leaveGroupRequest{
			group: m.LeaveGroupMessage.Group,
			ackID: m.LeaveGroupMessage.AckId,
		}, nil

	case *pb.UpstreamMessage_PingMessage_:
		return &pingRequest{}, nil

	default:
		return nil, fmt.Errorf("unknown message type: %T", m)
	}
}

// buildConnectedMessage creates a ConnectedMessage for initial connection.
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

// buildAckMessage creates an acknowledgment message.
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

// buildDataMessage creates a DataMessage with compressed protobuf payload.
// The compressedData should be Zstd-compressed protobuf bytes.
// typeUrl should be "proto.orderflow", "proto.gex", "proto.greek", etc.
func buildDataMessage(group string, compressedData []byte, typeUrl string) []byte {
	// Wrap in google.protobuf.Any with type URL matching real API
	anyMsg := &anypb.Any{
		TypeUrl: typeUrl,
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

// buildPongMessage creates a PongMessage response to client ping.
func buildPongMessage() []byte {
	msg := &pb.DownstreamMessage{
		Message: &pb.DownstreamMessage_PongMessage_{
			PongMessage: &pb.DownstreamMessage_PongMessage{},
		},
	}
	data, _ := proto.Marshal(msg)
	return data
}

// ============================================================================
// JSON Protocol Message Builders
// ============================================================================

// buildConnectedMessageJSON creates a JSON ConnectedMessage for Azure Web PubSub.
func buildConnectedMessageJSON(connectionID, userID string) []byte {
	msg := map[string]interface{}{
		"type":         "system",
		"event":        "connected",
		"connectionId": connectionID,
		"userId":       userID,
	}
	data, _ := json.Marshal(msg)
	return data
}

// buildAckMessageJSON creates a JSON acknowledgment message.
func buildAckMessageJSON(ackID uint64, success bool) []byte {
	msg := map[string]interface{}{
		"type":    "ack",
		"ackId":   ackID,
		"success": success,
	}
	data, _ := json.Marshal(msg)
	return data
}

// buildDataMessageJSON creates a JSON DataMessage with base64-encoded binary payload.
// The payload is wrapped in a google.protobuf.Any message to match protobuf protocol format.
// typeUrl should be "proto.orderflow", "proto.gex", "proto.greek", etc.
func buildDataMessageJSON(group string, encodedData []byte, typeUrl string) []byte {
	// Wrap in Any message (same as protobuf protocol) so Python client can parse uniformly
	anyMsg := &anypb.Any{
		TypeUrl: typeUrl,
		Value:   encodedData,
	}
	anyBytes, _ := proto.Marshal(anyMsg)

	msg := map[string]interface{}{
		"type":     "message",
		"from":     "group",
		"group":    group,
		"dataType": "binary",
		"data":     base64.StdEncoding.EncodeToString(anyBytes),
	}
	data, _ := json.Marshal(msg)
	return data
}

// buildDataMessageJSONRaw creates a JSON DataMessage with raw JSON payload.
// This matches the real GexBot API format for JSON clients - data is embedded directly
// as a JSON object rather than base64-encoded protobuf.
func buildDataMessageJSONRaw(group string, rawJSON json.RawMessage) []byte {
	msg := map[string]interface{}{
		"type":     "message",
		"from":     "group",
		"group":    group,
		"dataType": "json",
		"data":     rawJSON,
	}
	data, _ := json.Marshal(msg)
	return data
}

// buildPongMessageJSON creates a JSON PongMessage.
func buildPongMessageJSON() []byte {
	msg := map[string]interface{}{
		"type": "pong",
	}
	data, _ := json.Marshal(msg)
	return data
}

// parseUpstreamMessageJSON parses a JSON-encoded upstream message.
func parseUpstreamMessageJSON(data []byte) (any, error) {
	var msg map[string]interface{}
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, fmt.Errorf("unmarshal JSON upstream message: %w", err)
	}

	msgType, _ := msg["type"].(string)

	switch msgType {
	case "joinGroup":
		group, _ := msg["group"].(string)
		var ackID *uint64
		if v, ok := msg["ackId"].(float64); ok {
			id := uint64(v)
			ackID = &id
		}
		return &joinGroupRequest{group: group, ackID: ackID}, nil

	case "leaveGroup":
		group, _ := msg["group"].(string)
		var ackID *uint64
		if v, ok := msg["ackId"].(float64); ok {
			id := uint64(v)
			ackID = &id
		}
		return &leaveGroupRequest{group: group, ackID: ackID}, nil

	case "ping":
		return &pingRequest{}, nil

	default:
		return nil, fmt.Errorf("unknown JSON message type: %s", msgType)
	}
}
