package ws

import (
	"encoding/json"
	"fmt"

	"github.com/klauspost/compress/zstd"
	"google.golang.org/protobuf/proto"

	"github.com/dgnsrekt/gexbot-downloader/internal/data"
	ofpb "github.com/dgnsrekt/gexbot-downloader/internal/ws/generated/orderflow"
)

// Encoder converts JSON orderflow data to wire format (Protobuf + Zstd).
type Encoder struct {
	zstdEncoder *zstd.Encoder
}

// NewEncoder creates a new Encoder with Zstd compression.
func NewEncoder() (*Encoder, error) {
	enc, err := zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.SpeedDefault))
	if err != nil {
		return nil, fmt.Errorf("create zstd encoder: %w", err)
	}
	return &Encoder{zstdEncoder: enc}, nil
}

// EncodeOrderflow converts JSON orderflow data to Zstd-compressed protobuf.
// The result is ready to be wrapped in a DataMessage.
func (e *Encoder) EncodeOrderflow(jsonData []byte) ([]byte, error) {
	// 1. Parse JSON into OrderflowData
	var of data.OrderflowData
	if err := json.Unmarshal(jsonData, &of); err != nil {
		return nil, fmt.Errorf("unmarshal orderflow json: %w", err)
	}

	// 2. Convert to protobuf with integer scaling
	// Fields multiplied by 100: spot, gamma fields
	// Fields with no multiplier: state and orderflow fields
	pbMsg := &ofpb.Orderflow{
		Timestamp: of.Timestamp,
		Ticker:    of.Ticker,
		// Gamma fields: multiply by 100
		Spot:                uint32(of.Spot * 100),
		ZeroMajorLongGamma:  uint32(of.ZMlgamma * 100),
		ZeroMajorShortGamma: uint32(of.ZMsgamma * 100),
		OneMajorLongGamma:   uint32(of.OMlgamma * 100),
		OneMajorShortGamma:  uint32(of.OMsgamma * 100),
		ZeroMajorCallGamma:  uint32(of.ZeroMcall * 100),
		ZeroMajorPutGamma:   uint32(of.ZeroMput * 100),
		OneMajorCallGamma:   uint32(of.OneMcall * 100),
		OneMajorPutGamma:    uint32(of.OneMput * 100),
		// State fields: no multiplier (sint32)
		ZeroConvexityRatio: int32(of.Zcvr),
		OneConvexityRatio:  int32(of.Ocvr),
		ZeroGexRatio:       int32(of.Zgr),
		OneGexRatio:        int32(of.Ogr),
		ZeroNetVanna:       int32(of.Zvanna),
		OneNetVanna:        int32(of.Ovanna),
		ZeroNetCharm:       int32(of.Zcharm),
		OneNetCharm:        int32(of.Ocharm),
		ZeroAggTotalDex:    int32(of.AggDex),
		OneAggTotalDex:     int32(of.OneAggDex),
		ZeroAggCallDex:     int32(of.AggCallDex),
		OneAggCallDex:      int32(of.OneAggCallDex),
		ZeroAggPutDex:      int32(of.AggPutDex),
		OneAggPutDex:       int32(of.OneAggPutDex),
		ZeroNetTotalDex:    int32(of.NetDex),
		OneNetTotalDex:     int32(of.OneNetDex),
		ZeroNetCallDex:     int32(of.NetCallDex),
		OneNetCallDex:      int32(of.OneNetCallDex),
		ZeroNetPutDex:      int32(of.NetPutDex),
		OneNetPutDex:       int32(of.OneNetPutDex),
		// Orderflow fields: no multiplier (sint32)
		DexOrderflow:          int32(of.Dexoflow),
		GexOrderflow:          int32(of.Gexoflow),
		ConvexityOrderflow:    int32(of.Cvroflow),
		OneDexOrderflow:       int32(of.OneDexoflow),
		OneGexOrderflow:       int32(of.OneGexoflow),
		OneConvexityOrderflow: int32(of.OneCvroflow),
	}

	// 3. Serialize to protobuf bytes
	pbData, err := proto.Marshal(pbMsg)
	if err != nil {
		return nil, fmt.Errorf("marshal protobuf: %w", err)
	}

	// 4. Compress with Zstd
	compressed := e.zstdEncoder.EncodeAll(pbData, nil)

	return compressed, nil
}

// Close releases encoder resources.
func (e *Encoder) Close() {
	if e.zstdEncoder != nil {
		e.zstdEncoder.Close()
	}
}
