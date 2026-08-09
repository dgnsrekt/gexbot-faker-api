package ws

import (
	"encoding/json"
	"testing"

	pb "github.com/dgnsrekt/gexbot-downloader/internal/ws/generated/webpubsub"
	"google.golang.org/protobuf/proto"
)

// Both data-message builders emit the same logical group broadcast, so they must
// agree on the `from` field. These are group broadcasts, so the real Azure Web
// PubSub value is "group" (not "server").
func TestDataMessageFromParity(t *testing.T) {
	group := "blue_SPX_classic_gex_zero"

	var msg pb.DownstreamMessage
	if err := proto.Unmarshal(buildDataMessage(group, []byte("x"), "type.url/Foo"), &msg); err != nil {
		t.Fatal(err)
	}
	if got := msg.GetDataMessage().GetFrom(); got != "group" {
		t.Fatalf("protobuf from = %q, want \"group\"", got)
	}

	var m map[string]any
	if err := json.Unmarshal(buildDataMessageJSON(group, []byte("x"), "type.url/Foo"), &m); err != nil {
		t.Fatal(err)
	}
	if got := m["from"]; got != "group" {
		t.Fatalf("json from = %v, want \"group\"", got)
	}
}
