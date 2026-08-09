package server

import (
	"context"
	"fmt"
	"time"

	"github.com/dgnsrekt/gexbot-downloader/internal/api/generated"
)

// futuresMonthCode maps Jan..Dec to the CME month letter.
var futuresMonthCode = [12]byte{'F', 'G', 'H', 'J', 'K', 'M', 'N', 'Q', 'U', 'V', 'X', 'Z'}

// equityIndexFutures roll quarterly (Mar/Jun/Sep/Dec); others (GC, CL) are monthly.
var equityIndexFutures = map[string]bool{"ES": true, "NQ": true, "RTY": true, "YM": true}

// futurePair holds the fixed per-pair conversion constants. multiplier/additive
// are synthesized (the faker has no futures feed); basis is the spot-minus-future
// used by the additive model, nominalSpot is the reference price the other models
// are derived from, and affineSlope is a plausible fitted slope.
type futurePair struct {
	basis       float64
	affineSlope float64
	nominalSpot float64
}

// futurePairs is keyed "TICKER/FUTURE". SPX/ES and NDX/NQ use values observed
// from the live API; the rest are plausible constants.
var futurePairs = map[string]futurePair{
	"SPX/ES":  {basis: 23.6657268366, affineSlope: 0.683241221, nominalSpot: 7734.0},
	"SPY/ES":  {basis: 2.37, affineSlope: 0.68, nominalSpot: 773.0},
	"NDX/NQ":  {basis: 114.2872278159, affineSlope: 0.68, nominalSpot: 24000.0},
	"QQQ/NQ":  {basis: 2.8, affineSlope: 0.68, nominalSpot: 585.0},
	"RUT/RTY": {basis: 12.0, affineSlope: 0.68, nominalSpot: 2400.0},
	"IWM/RTY": {basis: 1.2, affineSlope: 0.68, nominalSpot: 240.0},
	"DIA/YM":  {basis: 20.0, affineSlope: 0.68, nominalSpot: 440.0},
	"GLD/GC":  {basis: 5.0, affineSlope: 1.0, nominalSpot: 305.0},
	"USO/CL":  {basis: 0.5, affineSlope: 1.0, nominalSpot: 80.0},
}

// defaultFuturePair is used for non-canonical ticker/future combinations.
var defaultFuturePair = futurePair{basis: 10.0, affineSlope: 0.68, nominalSpot: 1000.0}

// thirdFriday returns the standard monthly options/futures expiry (3rd Friday).
func thirdFriday(year int, month time.Month) time.Time {
	first := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
	offset := (int(time.Friday) - int(first.Weekday()) + 7) % 7
	return first.AddDate(0, 0, offset+14)
}

// frontMonthContract returns the active contract code (e.g. "ESU6") for a futures
// root as of anchor: the next quarterly expiry for equity-index futures, else the
// next calendar month.
func frontMonthContract(future string, anchor time.Time) string {
	var m time.Month
	var y int
	if equityIndexFutures[future] {
		m, y = nextQuarterly(anchor)
	} else {
		nm := anchor.AddDate(0, 1, 0)
		m, y = nm.Month(), nm.Year()
	}
	return fmt.Sprintf("%s%c%d", future, futuresMonthCode[m-1], y%10)
}

// nextQuarterly returns the month/year of the next quarterly expiry (3rd Friday of
// Mar/Jun/Sep/Dec) on or after anchor.
func nextQuarterly(anchor time.Time) (time.Month, int) {
	quarterly := []time.Month{time.March, time.June, time.September, time.December}
	for y := anchor.Year(); ; y++ {
		for _, qm := range quarterly {
			if !thirdFriday(y, qm).Before(anchor) {
				return qm, y
			}
		}
	}
}

// GetFuturesConversion implements generated.StrictServerInterface. It returns the
// active contract code (from the loaded date) and affine conversion parameters for
// the requested model, using a fixed per-pair basis.
func (s *Server) GetFuturesConversion(ctx context.Context, request generated.GetFuturesConversionRequestObject) (generated.GetFuturesConversionResponseObject, error) {
	// Match the live API's required-param errors (ticker checked first).
	if request.Params.Ticker == nil {
		return generated.GetFuturesConversion400JSONResponse{Error: ptr("ticker is required.")}, nil
	}
	if request.Params.Future == nil {
		return generated.GetFuturesConversion400JSONResponse{Error: ptr("future is required.")}, nil
	}
	ticker := string(*request.Params.Ticker)
	future := string(*request.Params.Future)
	model := "additive"
	if request.Params.Model != nil {
		model = string(*request.Params.Model)
	}

	anchor, err := time.Parse("2006-01-02", s.config.DataDate)
	if err != nil {
		return generated.GetFuturesConversion400JSONResponse{
			Error: ptr("current date is not a calendar date: " + s.config.DataDate),
		}, nil
	}

	pair, ok := futurePairs[ticker+"/"+future]
	if !ok {
		pair = defaultFuturePair
	}
	multiplier, additive := conversionParams(pair, model)

	return generated.GetFuturesConversion200JSONResponse{
		FutureContract: frontMonthContract(future, anchor),
		Multiplier:     multiplier,
		Additive:       additive,
	}, nil
}

// conversionParams derives the affine terms for a model from a fixed per-pair
// basis, consistently: additive uses the raw basis; multiplicative expresses the
// same basis as a ratio (additive 0); affine uses the pair's fitted slope with an
// intercept through the reference (future, spot) point.
func conversionParams(p futurePair, model string) (multiplier, additive float64) {
	futPrice := p.nominalSpot - p.basis
	switch model {
	case "multiplicative":
		return p.nominalSpot / futPrice, 0
	case "affine":
		return p.affineSlope, p.nominalSpot - p.affineSlope*futPrice
	default: // additive
		return 1, p.basis
	}
}
