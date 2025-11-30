package ws

import (
	"encoding/json"
	"fmt"

	"github.com/klauspost/compress/zstd"
	"google.golang.org/protobuf/proto"

	"github.com/dgnsrekt/gexbot-downloader/internal/data"
	gexpb "github.com/dgnsrekt/gexbot-downloader/internal/ws/generated/gex"
	greekpb "github.com/dgnsrekt/gexbot-downloader/internal/ws/generated/greek"
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

// EncodeGex converts JSON GEX data to Zstd-compressed protobuf.
// The result is ready to be wrapped in a DataMessage.
func (e *Encoder) EncodeGex(jsonData []byte) ([]byte, error) {
	// 1. Parse JSON into GexData
	var gex data.GexData
	if err := json.Unmarshal(jsonData, &gex); err != nil {
		return nil, fmt.Errorf("unmarshal gex json: %w", err)
	}

	// 2. Parse strikes array: [[strike_price, value_1, value_2, [priors]], ...]
	var rawStrikes [][]json.RawMessage
	if len(gex.Strikes) > 0 {
		if err := json.Unmarshal(gex.Strikes, &rawStrikes); err != nil {
			return nil, fmt.Errorf("unmarshal strikes: %w", err)
		}
	}

	pbStrikes := make([]*gexpb.Strike, 0, len(rawStrikes))
	for _, s := range rawStrikes {
		if len(s) < 3 {
			continue
		}
		var strikePrice, value1, value2 float64
		json.Unmarshal(s[0], &strikePrice)
		json.Unmarshal(s[1], &value1)
		json.Unmarshal(s[2], &value2)

		strike := &gexpb.Strike{
			StrikePrice: uint32(strikePrice * 100),
			Value_1:     int32(value1 * 100),
			Value_2:     int32(value2 * 100),
		}

		// Parse priors if present
		if len(s) >= 4 {
			var priors []float64
			if err := json.Unmarshal(s[3], &priors); err == nil && len(priors) > 0 {
				priorValues := make([]int32, len(priors))
				for i, p := range priors {
					priorValues[i] = int32(p * 100)
				}
				strike.Priors = &gexpb.Priors{Values: priorValues}
			}
		}
		pbStrikes = append(pbStrikes, strike)
	}

	// 3. Parse max_priors: [[first, second], ...] (6 tuples)
	var rawMaxPriors [][]float64
	var pbMaxPriors *gexpb.MaxPriors
	if len(gex.MaxPriors) > 0 {
		if err := json.Unmarshal(gex.MaxPriors, &rawMaxPriors); err == nil && len(rawMaxPriors) > 0 {
			tuples := make([]*gexpb.MaxPriorsTuple, 0, len(rawMaxPriors))
			for _, mp := range rawMaxPriors {
				if len(mp) >= 2 {
					tuples = append(tuples, &gexpb.MaxPriorsTuple{
						FirstValue:  int32(mp[0] * 100),
						SecondValue: int32(mp[1] * 1000),
					})
				}
			}
			if len(tuples) > 0 {
				pbMaxPriors = &gexpb.MaxPriors{Tuples: tuples}
			}
		}
	}

	// 4. Build protobuf message with integer scaling
	minDte := int32(gex.MinDTE)
	secMinDte := int32(gex.SecMinDTE)

	pbMsg := &gexpb.Gex{
		Timestamp:  gex.Timestamp,
		Ticker:     gex.Ticker,
		MinDte:     &minDte,
		SecMinDte:  &secMinDte,
		// Fields multiplied by 100
		Spot:        uint32(gex.Spot * 100),
		ZeroGamma:   uint32(gex.ZeroGamma * 100),
		MajorPosVol: uint32(gex.MajorPosVol * 100),
		MajorPosOi:  uint32(gex.MajorPosOI * 100),
		MajorNegVol: uint32(gex.MajorNegVol * 100),
		MajorNegOi:  uint32(gex.MajorNegOI * 100),
		Strikes:     pbStrikes,
		// Fields multiplied by 1000
		SumGexVol:         int32(gex.SumGexVol * 1000),
		SumGexOi:          int32(gex.SumGexOI * 1000),
		DeltaRiskReversal: int32(gex.DeltaRiskReversal * 1000),
		MaxPriors:         pbMaxPriors,
	}

	// 5. Serialize to protobuf bytes
	pbData, err := proto.Marshal(pbMsg)
	if err != nil {
		return nil, fmt.Errorf("marshal gex protobuf: %w", err)
	}

	// 6. Compress with Zstd
	compressed := e.zstdEncoder.EncodeAll(pbData, nil)

	return compressed, nil
}

// EncodeGreek converts JSON Greek data to Zstd-compressed protobuf.
// The result is ready to be wrapped in a DataMessage.
func (e *Encoder) EncodeGreek(jsonData []byte) ([]byte, error) {
	// 1. Parse JSON into GreekData
	var greek data.GreekData
	if err := json.Unmarshal(jsonData, &greek); err != nil {
		return nil, fmt.Errorf("unmarshal greek json: %w", err)
	}

	// 2. Parse mini_contracts: [[strike, call_ivol, put_ivol, call_vol, priors, put_vol, put_priors], ...]
	var rawContracts [][]json.RawMessage
	if len(greek.MiniContracts) > 0 {
		if err := json.Unmarshal(greek.MiniContracts, &rawContracts); err != nil {
			return nil, fmt.Errorf("unmarshal mini_contracts: %w", err)
		}
	}

	pbContracts := make([]*greekpb.MiniContract, 0, len(rawContracts))
	for _, c := range rawContracts {
		if len(c) < 5 {
			continue
		}

		// Parse required fields
		var strike, callIvol, putIvol, callCvolume float64
		json.Unmarshal(c[0], &strike)
		json.Unmarshal(c[1], &callIvol)
		json.Unmarshal(c[2], &putIvol)
		json.Unmarshal(c[3], &callCvolume)

		contract := &greekpb.MiniContract{
			Strike:      uint32(strike * 100),
			CallIvol:    uint32(callIvol * 1000),
			PutIvol:     uint32(putIvol * 1000),
			CallCvolume: int32(callCvolume * 100),
		}

		// Parse call_cvolume_priors (index 4) - array of floats × 100
		var callPriors []float64
		if err := json.Unmarshal(c[4], &callPriors); err == nil && len(callPriors) > 0 {
			priorValues := make([]int32, len(callPriors))
			for i, p := range callPriors {
				priorValues[i] = int32(p * 100)
			}
			contract.CallCvolumePriors = priorValues
		}

		// Parse optional put_cvolume (index 5) - can be null or number, no multiplier
		if len(c) >= 6 {
			var putCvolume *float64
			if err := json.Unmarshal(c[5], &putCvolume); err == nil && putCvolume != nil {
				pv := int32(*putCvolume)
				contract.PutCvolume = &pv
			}
		}

		// Parse optional put_cvolume_priors (index 6) - can be null or array of ints, no multiplier
		if len(c) >= 7 {
			var putPriors []int32
			if err := json.Unmarshal(c[6], &putPriors); err == nil && len(putPriors) > 0 {
				contract.PutCvolumePriors = &greekpb.MiniContractPriors{Values: putPriors}
			}
		}

		pbContracts = append(pbContracts, contract)
	}

	// 3. Build OptionProfile protobuf message with integer scaling
	minDte := int32(greek.MinDTE)
	secMinDte := int32(greek.SecMinDTE)

	pbMsg := &greekpb.OptionProfile{
		Timestamp:       greek.Timestamp,
		Ticker:          greek.Ticker,
		Spot:            uint32(greek.Spot * 100),
		MinDte:          &minDte,
		SecMinDte:       &secMinDte,
		MajorCallGamma:  uint32(greek.MajorPositive * 100),
		MajorPutGamma:   uint32(greek.MajorNegative * 100),
		MajorLongGamma:  uint32(greek.MajorLongGamma * 100),
		MajorShortGamma: uint32(greek.MajorShortGamma * 100),
		MiniContracts:   pbContracts,
	}

	// 4. Serialize to protobuf bytes
	pbData, err := proto.Marshal(pbMsg)
	if err != nil {
		return nil, fmt.Errorf("marshal greek protobuf: %w", err)
	}

	// 5. Compress with Zstd
	compressed := e.zstdEncoder.EncodeAll(pbData, nil)

	return compressed, nil
}

// Close releases encoder resources.
func (e *Encoder) Close() {
	if e.zstdEncoder != nil {
		e.zstdEncoder.Close()
	}
}
