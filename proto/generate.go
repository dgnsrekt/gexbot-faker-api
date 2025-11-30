// Package proto contains protobuf definitions for WebSocket wire protocol.
package proto

//go:generate protoc --proto_path=. --proto_path=$HOME/bin/include --go_out=../internal/ws/generated/orderflow --go_opt=paths=source_relative orderflow.proto
//go:generate protoc --proto_path=. --proto_path=$HOME/bin/include --go_out=../internal/ws/generated/webpubsub --go_opt=paths=source_relative webpubsub_messages.proto
