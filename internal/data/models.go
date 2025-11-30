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
