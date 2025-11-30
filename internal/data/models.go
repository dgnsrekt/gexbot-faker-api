package data

import "encoding/json"

type GexData struct {
	Timestamp         int64           `json:"timestamp"`
	Ticker            string          `json:"ticker"`
	MinDTE            int             `json:"min_dte"`
	SecMinDTE         int             `json:"sec_min_dte"`
	Spot              float64         `json:"spot"`
	ZeroGamma         float64         `json:"zero_gamma"`
	MajorPosVol       float64         `json:"major_pos_vol"`
	MajorPosOI        float64         `json:"major_pos_oi"`
	MajorNegVol       float64         `json:"major_neg_vol"`
	MajorNegOI        float64         `json:"major_neg_oi"`
	Strikes           json.RawMessage `json:"strikes"`
	SumGexVol         float64         `json:"sum_gex_vol"`
	SumGexOI          float64         `json:"sum_gex_oi"`
	DeltaRiskReversal float64         `json:"delta_risk_reversal"`
	MaxPriors         json.RawMessage `json:"max_priors"`
}

// GreekData represents options profile Greeks data (delta, gamma, charm, vanna)
type GreekData struct {
	Timestamp       int64           `json:"timestamp"`
	Ticker          string          `json:"ticker"`
	Spot            float64         `json:"spot"`
	MinDTE          int             `json:"min_dte"`
	SecMinDTE       int             `json:"sec_min_dte"`
	MajorPositive   float64         `json:"major_positive"`
	MajorNegative   float64         `json:"major_negative"`
	MajorLongGamma  float64         `json:"major_long_gamma"`
	MajorShortGamma float64         `json:"major_short_gamma"`
	MiniContracts   json.RawMessage `json:"mini_contracts"`
}

// OrderflowData represents real-time orderflow metrics for nearest and next expiries
type OrderflowData struct {
	Timestamp     int64   `json:"timestamp"`
	Ticker        string  `json:"ticker"`
	Spot          float64 `json:"spot"`
	ZMlgamma      float64 `json:"z_mlgamma"`
	ZMsgamma      float64 `json:"z_msgamma"`
	OMlgamma      float64 `json:"o_mlgamma"`
	OMsgamma      float64 `json:"o_msgamma"`
	ZeroMcall     float64 `json:"zero_mcall"`
	ZeroMput      float64 `json:"zero_mput"`
	OneMcall      float64 `json:"one_mcall"`
	OneMput       float64 `json:"one_mput"`
	Zcvr          float64 `json:"zcvr"`
	Ocvr          float64 `json:"ocvr"`
	Zgr           float64 `json:"zgr"`
	Ogr           float64 `json:"ogr"`
	Zvanna        float64 `json:"zvanna"`
	Ovanna        float64 `json:"ovanna"`
	Zcharm        float64 `json:"zcharm"`
	Ocharm        float64 `json:"ocharm"`
	AggDex        float64 `json:"agg_dex"`
	OneAggDex     float64 `json:"one_agg_dex"`
	AggCallDex    float64 `json:"agg_call_dex"`
	OneAggCallDex float64 `json:"one_agg_call_dex"`
	AggPutDex     float64 `json:"agg_put_dex"`
	OneAggPutDex  float64 `json:"one_agg_put_dex"`
	NetDex        float64 `json:"net_dex"`
	OneNetDex     float64 `json:"one_net_dex"`
	NetCallDex    float64 `json:"net_call_dex"`
	OneNetCallDex float64 `json:"one_net_call_dex"`
	NetPutDex     float64 `json:"net_put_dex"`
	OneNetPutDex  float64 `json:"one_net_put_dex"`
	Dexoflow      float64 `json:"dexoflow"`
	Gexoflow      float64 `json:"gexoflow"`
	Cvroflow      float64 `json:"cvroflow"`
	OneDexoflow   float64 `json:"one_dexoflow"`
	OneGexoflow   float64 `json:"one_gexoflow"`
	OneCvroflow   float64 `json:"one_cvroflow"`
}
