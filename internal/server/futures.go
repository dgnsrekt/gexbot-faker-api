package server

import (
	"context"
	"fmt"
	"time"

	"github.com/dgnsrekt/gexbot-downloader/internal/api/generated"
)

// futuresMonthCode maps Jan..Dec to the CME month letter.
var futuresMonthCode = [12]byte{'F', 'G', 'H', 'J', 'K', 'M', 'N', 'Q', 'U', 'V', 'X', 'Z'}

// futurePair holds the fixed per-pair conversion constants. multiplier/additive
// are synthesized (the faker has no futures feed); basis is the spot-minus-future
// used by the additive model, nominalSpot is the reference price the other models
// are derived from, and affineSlope is a plausible fitted slope.
type futurePair struct {
	basis       float64
	affineSlope float64
	nominalSpot float64
}

// futurePairs is keyed "TICKER/FUTURE" and enumerates the only meaningful
// conversion pairs. SPX/ES and NDX/NQ use values observed from the live API; the
// rest are plausible constants. A ticker/future combo not present here is an
// unsupported pair (400), matching the live API.
var futurePairs = map[string]futurePair{
	"SPX/ES":  {basis: 23.6657268366, affineSlope: 0.683241221, nominalSpot: 7734.0},
	"SPY/ES":  {basis: 2.37, affineSlope: 0.68, nominalSpot: 773.0},
	"NDX/NQ":  {basis: 114.2872278159, affineSlope: 0.68, nominalSpot: 24000.0},
	"QQQ/NQ":  {basis: 2.8, affineSlope: 0.68, nominalSpot: 585.0},
	"RUT/RTY": {basis: 7.7961446465, affineSlope: 0.68, nominalSpot: 2400.0},
	"IWM/RTY": {basis: 0.78, affineSlope: 0.68, nominalSpot: 240.0},
	"DIA/YM":  {basis: 53.6, affineSlope: 0.68, nominalSpot: 440.0},
	"GLD/GC":  {basis: 5.0, affineSlope: 1.0, nominalSpot: 305.0},
	"USO/CL":  {basis: 0.5, affineSlope: 1.0, nominalSpot: 80.0},
}

// thirdFriday returns the standard monthly options/futures expiry (3rd Friday).
func thirdFriday(year int, month time.Month) time.Time {
	first := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
	offset := (int(time.Friday) - int(first.Weekday()) + 7) % 7
	return first.AddDate(0, 0, offset+14)
}

// frontMonthContract returns the active contract code (e.g. "ESU6") for a futures
// root as of anchor, using per-root roll rules (equity-index vs crude vs gold).
func frontMonthContract(future string, anchor time.Time) string {
	var m time.Month
	var y int
	switch future {
	case "ES", "NQ", "RTY", "YM":
		m, y = equityIndexFront(anchor)
	case "CL":
		m, y = crudeFront(anchor)
	case "GC":
		m, y = goldFront(anchor)
	default:
		m, y = anchor.Month(), anchor.Year() // unreachable: only canonical pairs reach here
	}
	return fmt.Sprintf("%s%c%d", future, futuresMonthCode[m-1], y%10)
}

// equityIndexFront returns the front quarterly contract (Mar/Jun/Sep/Dec).
// Equity-index liquidity rolls ~8 days before the third-Friday expiry, so the
// front advances to the next quarter once anchor passes that roll date.
func equityIndexFront(anchor time.Time) (time.Month, int) {
	quarterly := []time.Month{time.March, time.June, time.September, time.December}
	for y := anchor.Year(); ; y++ {
		for _, qm := range quarterly {
			roll := thirdFriday(y, qm).AddDate(0, 0, -8)
			if !roll.Before(anchor) {
				return qm, y
			}
		}
	}
}

// clTermination returns the last trading day of the CL contract for the given
// delivery month: the 3rd business day before the 25th calendar day of the month
// prior to delivery (if the 25th is not a business day, count from the business
// day preceding it). Per CME NYMEX rule 200.
func clTermination(deliveryYear int, deliveryMonth time.Month) time.Time {
	prior := time.Date(deliveryYear, deliveryMonth, 1, 0, 0, 0, 0, time.UTC).AddDate(0, -1, 0)
	ref := time.Date(prior.Year(), prior.Month(), 25, 0, 0, 0, 0, time.UTC)
	for !isMarketDay(ref) {
		ref = ref.AddDate(0, 0, -1)
	}
	for count := 0; count < 3; {
		ref = ref.AddDate(0, 0, -1)
		if isMarketDay(ref) {
			count++
		}
	}
	return ref
}

// crudeFront returns the front CL delivery month: the earliest delivery month
// whose termination day is on or after anchor.
func crudeFront(anchor time.Time) (time.Month, int) {
	d := time.Date(anchor.Year(), anchor.Month(), 1, 0, 0, 0, 0, time.UTC)
	for {
		if !clTermination(d.Year(), d.Month()).Before(anchor) {
			return d.Month(), d.Year()
		}
		d = d.AddDate(0, 1, 0)
	}
}

// goldFront returns the front GC delivery month. Gold liquidity concentrates in
// Feb/Apr/Jun/Aug/Dec; a contract stops being front once its delivery month
// begins, so the front is the next liquid month starting after anchor (e.g. early
// August rolls straight to December).
func goldFront(anchor time.Time) (time.Month, int) {
	liquid := []time.Month{time.February, time.April, time.June, time.August, time.December}
	for y := anchor.Year(); ; y++ {
		for _, gm := range liquid {
			if time.Date(y, gm, 1, 0, 0, 0, 0, time.UTC).After(anchor) {
				return gm, y
			}
		}
	}
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

// GetFuturesConversion implements generated.StrictServerInterface. It returns the
// active contract code (from the loaded date) and affine conversion parameters for
// the requested model. ticker/future are required (validated by the spec); an
// unsupported pair returns 400 like the live API.
func (s *Server) GetFuturesConversion(ctx context.Context, request generated.GetFuturesConversionRequestObject) (generated.GetFuturesConversionResponseObject, error) {
	ticker := string(request.Params.Ticker)
	future := string(request.Params.Future)
	model := "additive"
	if request.Params.Model != nil {
		model = string(*request.Params.Model)
	}

	pair, ok := futurePairs[ticker+"/"+future]
	if !ok {
		return generated.GetFuturesConversion400JSONResponse{
			Error: ptr(fmt.Sprintf("Unsupported futures conversion pair: %s/%s.", ticker, future)),
		}, nil
	}

	anchor, err := time.Parse("2006-01-02", s.config.DataDate)
	if err != nil {
		return generated.GetFuturesConversion400JSONResponse{
			Error: ptr("current date is not a calendar date: " + s.config.DataDate),
		}, nil
	}
	multiplier, additive := conversionParams(pair, model)

	return generated.GetFuturesConversion200JSONResponse{
		FutureContract: frontMonthContract(future, anchor),
		Multiplier:     multiplier,
		Additive:       additive,
	}, nil
}
