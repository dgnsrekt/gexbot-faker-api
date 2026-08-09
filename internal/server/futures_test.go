package server

import (
	"context"
	"math"
	"testing"

	"github.com/dgnsrekt/gexbot-downloader/internal/api/generated"
	"github.com/dgnsrekt/gexbot-downloader/internal/config"
)

func TestFrontMonthContract(t *testing.T) {
	cases := []struct {
		future, date, want string
	}{
		// Equity index: quarterly, front = Sep (U6) in early August (matches live).
		{"ES", "2026-08-07", "ESU6"},
		{"NQ", "2026-08-07", "NQU6"},
		{"RTY", "2026-08-07", "RTYU6"},
		{"YM", "2026-08-07", "YMU6"},
		// Equity-index roll ~8 days before the third Friday (2026-09-18 -> roll 09-10).
		{"ES", "2026-09-10", "ESU6"}, // roll day: still front
		{"ES", "2026-09-11", "ESZ6"}, // after roll: next quarter (Dec)
		{"ES", "2026-12-31", "ESH7"}, // past Dec roll -> next-year March
		// Crude: business-day termination (3 bd before the 25th of the prior month).
		// Oct-delivery CL terminates 3 bd before Fri 2026-09-25 = Tue 2026-09-22.
		{"CL", "2026-08-07", "CLU6"}, // live: Sep contract
		{"CL", "2026-09-22", "CLV6"}, // termination day: Oct still front
		{"CL", "2026-09-23", "CLX6"}, // after termination: rolls to Nov
		// Gold: liquid Feb/Apr/Jun/Aug/Dec; early August rolls to Dec (live: GCZ6).
		{"GC", "2026-08-07", "GCZ6"},
		{"GC", "2026-07-31", "GCQ6"}, // before Aug delivery begins
		{"GC", "2026-08-01", "GCZ6"}, // Aug delivery started -> next liquid (Dec)
	}
	for _, tc := range cases {
		if got := frontMonthContract(tc.future, mustDate(t, tc.date)); got != tc.want {
			t.Errorf("%s @ %s: got %s, want %s", tc.future, tc.date, got, tc.want)
		}
	}
}

func TestConversionParams(t *testing.T) {
	p := futurePairs["SPX/ES"] // observed live constants
	fut := p.nominalSpot - p.basis

	if m, a := conversionParams(p, "additive"); m != 1 || math.Abs(a-23.6657268366) > 1e-6 {
		t.Errorf("additive: got mult=%v add=%v", m, a)
	}
	m, a := conversionParams(p, "multiplicative")
	if a != 0 || math.Abs(m-p.nominalSpot/fut) > 1e-9 || m < 1.002 || m > 1.004 {
		t.Errorf("multiplicative: got mult=%v add=%v", m, a)
	}
	m, a = conversionParams(p, "affine")
	if m != p.affineSlope || math.Abs((m*fut+a)-p.nominalSpot) > 1e-6 {
		t.Errorf("affine line must pass through (future, spot): mult=%v add=%v", m, a)
	}
	if m, a := conversionParams(p, "bogus"); m != 1 || a != p.basis {
		t.Errorf("unknown model should default to additive: mult=%v add=%v", m, a)
	}
}

func futuresRequest(ticker, future string) generated.GetFuturesConversionRequestObject {
	return generated.GetFuturesConversionRequestObject{
		Params: generated.GetFuturesConversionParams{
			Ticker: generated.GetFuturesConversionParamsTicker(ticker),
			Future: generated.GetFuturesConversionParamsFuture(future),
		},
	}
}

func TestFuturesConversionUnsupportedPair(t *testing.T) {
	srv := &Server{config: &config.ServerConfig{DataDate: "2026-08-07"}}
	for _, pair := range [][2]string{{"SPX", "CL"}, {"GLD", "ES"}} {
		resp, err := srv.GetFuturesConversion(context.Background(), futuresRequest(pair[0], pair[1]))
		if err != nil {
			t.Fatal(err)
		}
		got, ok := resp.(generated.GetFuturesConversion400JSONResponse)
		if !ok {
			t.Fatalf("%s/%s: expected 400, got %T", pair[0], pair[1], resp)
		}
		want := "Unsupported futures conversion pair: " + pair[0] + "/" + pair[1] + "."
		if got.Error == nil || *got.Error != want {
			t.Errorf("%s/%s: error body = %v, want %q", pair[0], pair[1], got.Error, want)
		}
	}
}

func TestFuturesConversionCanonicalPair(t *testing.T) {
	srv := &Server{config: &config.ServerConfig{DataDate: "2026-08-07"}}
	resp, err := srv.GetFuturesConversion(context.Background(), futuresRequest("SPX", "ES"))
	if err != nil {
		t.Fatal(err)
	}
	got, ok := resp.(generated.GetFuturesConversion200JSONResponse)
	if !ok {
		t.Fatalf("expected 200, got %T", resp)
	}
	if got.FutureContract != "ESU6" {
		t.Errorf("future_contract = %s, want ESU6", got.FutureContract)
	}
	if got.Multiplier != 1 || math.Abs(got.Additive-23.6657268366) > 1e-6 {
		t.Errorf("additive model: mult=%v add=%v", got.Multiplier, got.Additive)
	}
}
