package server

import (
	"math"
	"testing"
)

func TestFrontMonthContract(t *testing.T) {
	aug := mustDate(t, "2026-08-07")
	// Equity-index futures roll to the next quarterly (Sep 2026 = U6).
	for _, tc := range []struct{ future, want string }{
		{"ES", "ESU6"}, {"NQ", "NQU6"}, {"RTY", "RTYU6"}, {"YM", "YMU6"},
	} {
		if got := frontMonthContract(tc.future, aug); got != tc.want {
			t.Errorf("%s @ 2026-08-07: got %s, want %s", tc.future, got, tc.want)
		}
	}
	// After the Sep expiry the quarterly rolls to Dec (Z6).
	if got := frontMonthContract("ES", mustDate(t, "2026-09-30")); got != "ESZ6" {
		t.Errorf("post-Sep roll: got %s, want ESZ6", got)
	}
	// Across the year boundary rolls to next March (H7).
	if got := frontMonthContract("ES", mustDate(t, "2026-12-31")); got != "ESH7" {
		t.Errorf("year-boundary roll: got %s, want ESH7", got)
	}
	// Commodities are monthly: next calendar month.
	if got := frontMonthContract("CL", aug); got != "CLU6" {
		t.Errorf("CL @ 2026-08-07: got %s, want CLU6", got)
	}
}

func TestConversionParams(t *testing.T) {
	p := futurePairs["SPX/ES"] // observed live constants
	fut := p.nominalSpot - p.basis

	// additive: multiplier 1, additive = basis (matches live probe 23.6657…).
	if m, a := conversionParams(p, "additive"); m != 1 || math.Abs(a-23.6657268366) > 1e-6 {
		t.Errorf("additive: got mult=%v add=%v", m, a)
	}
	// multiplicative: additive 0, multiplier = spot/future (~1.0031, live 1.00305).
	m, a := conversionParams(p, "multiplicative")
	if a != 0 || math.Abs(m-p.nominalSpot/fut) > 1e-9 || m < 1.002 || m > 1.004 {
		t.Errorf("multiplicative: got mult=%v add=%v", m, a)
	}
	// affine: multiplier = fitted slope; intercept passes through (future, spot).
	m, a = conversionParams(p, "affine")
	if m != p.affineSlope || math.Abs((m*fut+a)-p.nominalSpot) > 1e-6 {
		t.Errorf("affine line must pass through (future, spot): mult=%v add=%v", m, a)
	}
	// unknown model falls back to additive.
	if m, a := conversionParams(p, "bogus"); m != 1 || a != p.basis {
		t.Errorf("unknown model should default to additive: mult=%v add=%v", m, a)
	}
}
