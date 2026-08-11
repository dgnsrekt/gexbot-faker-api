package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/dgnsrekt/gexbot-downloader/internal/api/generated"
	"github.com/dgnsrekt/gexbot-downloader/internal/config"
	"github.com/dgnsrekt/gexbot-downloader/internal/data"
	"github.com/dgnsrekt/gexbot-downloader/internal/eod"
)

// Custom response types for GetStateProfile oneOf responses
type stateProfileGexDataResponse generated.GexData

func (r stateProfileGexDataResponse) VisitGetStateProfileResponse(w http.ResponseWriter) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	return json.NewEncoder(w).Encode(r)
}

type stateProfileGreekDataResponse generated.GreekProfileData

func (r stateProfileGreekDataResponse) VisitGetStateProfileResponse(w http.ResponseWriter) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	return json.NewEncoder(w).Encode(r)
}

type Server struct {
	loader        data.DataLoader
	cache         *data.IndexCache
	config        *config.ServerConfig
	logger        *zap.Logger
	loadedAt      time.Time
	reloadManager *ReloadManager
	rangeLoad     *rangeLoadManager
}

func NewServer(loader data.DataLoader, cache *data.IndexCache, cfg *config.ServerConfig, logger *zap.Logger, reloadManager *ReloadManager) *Server {
	return &Server{
		loader:        loader,
		cache:         cache,
		config:        cfg,
		logger:        logger,
		loadedAt:      time.Now(),
		reloadManager: reloadManager,
		rangeLoad:     newRangeLoadManager(cfg.DataDir, reloadManager, logger),
	}
}

// Compile-time interface verification
var _ generated.StrictServerInterface = (*Server)(nil)

// GetClassicGexMajors implements generated.StrictServerInterface
func (s *Server) GetClassicGexMajors(ctx context.Context, request generated.GetClassicGexMajorsRequestObject) (generated.GetClassicGexMajorsResponseObject, error) {
	ticker := request.Ticker
	aggregation := string(request.Aggregation)
	apiKey := authKeyFromContext(ctx)

	// Map aggregation to internal category format
	category := aggregation // path segment already carries the gex_ prefix (gex_full/gex_zero/gex_one)
	pkg := "classic"

	s.logger.Debug("classic gex majors request",
		zap.String("ticker", ticker),
		zap.String("aggregation", aggregation),
		zap.String("category", category),
		zap.String("apiKey", maskAPIKey(apiKey)),
	)

	// Check if data exists
	if !s.loader.Exists(ticker, pkg, category) {
		return generated.GetClassicGexMajors404JSONResponse{
			Error: ptr("Data not found for " + ticker + "/classic/" + aggregation),
		}, nil
	}

	// Get data length
	length, err := s.loader.GetLength(ticker, pkg, category)
	if err != nil {
		return generated.GetClassicGexMajors404JSONResponse{
			Error: ptr(err.Error()),
		}, nil
	}

	// Build cache key based on endpoint cache mode
	var cacheKey string
	if s.config.EndpointCacheMode == "shared" {
		cacheKey = data.SharedCacheKey(ticker, pkg, apiKey)
	} else {
		// Independent mode - include category with _majors suffix
		cacheKey = data.CacheKey(ticker, pkg, category+"_majors", apiKey)
	}
	idx, exhausted := s.cache.GetAndAdvance(cacheKey, length)

	if exhausted {
		s.logger.Debug("data exhausted",
			zap.String("cacheKey", maskCacheKey(cacheKey)),
			zap.Int("index", idx),
			zap.Int("length", length),
		)
		return generated.GetClassicGexMajors404JSONResponse{
			Error: ptr("No more data available"),
		}, nil
	}

	// Get data at index
	gexData, err := s.loader.GetAtIndex(ctx, ticker, pkg, category, idx)
	if err != nil {
		if errors.Is(err, data.ErrIndexOutOfBounds) {
			return generated.GetClassicGexMajors404JSONResponse{
				Error: ptr("Index out of bounds"),
			}, nil
		}
		return generated.GetClassicGexMajors404JSONResponse{
			Error: ptr(err.Error()),
		}, nil
	}

	s.logger.Debug("returning majors data",
		zap.String("cacheKey", maskCacheKey(cacheKey)),
		zap.Int("index", idx),
		zap.Int64("timestamp", gexData.Timestamp),
	)

	return generated.GetClassicGexMajors200JSONResponse{
		Timestamp: gexData.Timestamp,
		Ticker:    gexData.Ticker,
		Spot:      &gexData.Spot,
		MposVol:   &gexData.MajorPosVol,
		MposOi:    &gexData.MajorPosOI,
		MnegVol:   &gexData.MajorNegVol,
		MnegOi:    &gexData.MajorNegOI,
		ZeroGamma: &gexData.ZeroGamma,
		NetGexVol: &gexData.SumGexVol,
		NetGexOi:  &gexData.SumGexOI,
	}, nil
}

// GetClassicGexMaxChange implements generated.StrictServerInterface
func (s *Server) GetClassicGexMaxChange(ctx context.Context, request generated.GetClassicGexMaxChangeRequestObject) (generated.GetClassicGexMaxChangeResponseObject, error) {
	ticker := request.Ticker
	aggregation := string(request.Aggregation)
	apiKey := authKeyFromContext(ctx)

	// Map aggregation to internal category format
	category := aggregation // path segment already carries the gex_ prefix (gex_full/gex_zero/gex_one)
	pkg := "classic"

	s.logger.Debug("classic gex max change request",
		zap.String("ticker", ticker),
		zap.String("aggregation", aggregation),
		zap.String("category", category),
		zap.String("apiKey", maskAPIKey(apiKey)),
	)

	// Check if data exists
	if !s.loader.Exists(ticker, pkg, category) {
		return generated.GetClassicGexMaxChange404JSONResponse{
			Error: ptr("Data not found for " + ticker + "/classic/" + aggregation),
		}, nil
	}

	// Get data length
	length, err := s.loader.GetLength(ticker, pkg, category)
	if err != nil {
		return generated.GetClassicGexMaxChange404JSONResponse{
			Error: ptr(err.Error()),
		}, nil
	}

	// Build cache key based on endpoint cache mode
	var cacheKey string
	if s.config.EndpointCacheMode == "shared" {
		cacheKey = data.SharedCacheKey(ticker, pkg, apiKey)
	} else {
		// Independent mode - include category with _maxchange suffix
		cacheKey = data.CacheKey(ticker, pkg, category+"_maxchange", apiKey)
	}
	idx, exhausted := s.cache.GetAndAdvance(cacheKey, length)

	if exhausted {
		s.logger.Debug("data exhausted",
			zap.String("cacheKey", maskCacheKey(cacheKey)),
			zap.Int("index", idx),
			zap.Int("length", length),
		)
		return generated.GetClassicGexMaxChange404JSONResponse{
			Error: ptr("No more data available"),
		}, nil
	}

	// Get data at index
	gexData, err := s.loader.GetAtIndex(ctx, ticker, pkg, category, idx)
	if err != nil {
		if errors.Is(err, data.ErrIndexOutOfBounds) {
			return generated.GetClassicGexMaxChange404JSONResponse{
				Error: ptr("Index out of bounds"),
			}, nil
		}
		return generated.GetClassicGexMaxChange404JSONResponse{
			Error: ptr(err.Error()),
		}, nil
	}

	// Parse max_priors: [[strike, gex], [strike, gex], ...] (6 pairs)
	var maxPriors [][]float32
	if gexData.MaxPriors != nil {
		if err := json.Unmarshal(gexData.MaxPriors, &maxPriors); err != nil {
			s.logger.Warn("failed to unmarshal max_priors", zap.Error(err))
		}
	}

	s.logger.Debug("returning max change data",
		zap.String("cacheKey", maskCacheKey(cacheKey)),
		zap.Int("index", idx),
		zap.Int64("timestamp", gexData.Timestamp),
	)

	// Map to response fields (ensure we have 6 elements)
	response := generated.GetClassicGexMaxChange200JSONResponse{
		Timestamp: gexData.Timestamp,
		Ticker:    gexData.Ticker,
	}

	if len(maxPriors) >= 6 {
		response.Current = &maxPriors[0]
		response.One = &maxPriors[1]
		response.Five = &maxPriors[2]
		response.Ten = &maxPriors[3]
		response.Fifteen = &maxPriors[4]
		response.Thirty = &maxPriors[5]
	}

	return response, nil
}

// GetClassicGexChain implements generated.StrictServerInterface
func (s *Server) GetClassicGexChain(ctx context.Context, request generated.GetClassicGexChainRequestObject) (generated.GetClassicGexChainResponseObject, error) {
	ticker := request.Ticker
	aggregation := string(request.Aggregation)
	apiKey := authKeyFromContext(ctx)

	// Map aggregation to internal category format
	category := aggregation // path segment already carries the gex_ prefix (gex_full/gex_zero/gex_one)
	pkg := "classic"

	s.logger.Debug("classic gex chain request",
		zap.String("ticker", ticker),
		zap.String("aggregation", aggregation),
		zap.String("category", category),
		zap.String("apiKey", maskAPIKey(apiKey)),
	)

	// Check if data exists
	if !s.loader.Exists(ticker, pkg, category) {
		return generated.GetClassicGexChain404JSONResponse{
			Error: ptr("Data not found for " + ticker + "/classic/" + aggregation),
		}, nil
	}

	// Get data length
	length, err := s.loader.GetLength(ticker, pkg, category)
	if err != nil {
		return generated.GetClassicGexChain404JSONResponse{
			Error: ptr(err.Error()),
		}, nil
	}

	// Build cache key based on endpoint cache mode
	var cacheKey string
	if s.config.EndpointCacheMode == "shared" {
		cacheKey = data.SharedCacheKey(ticker, pkg, apiKey)
	} else {
		// Independent mode - include category
		cacheKey = data.CacheKey(ticker, pkg, category, apiKey)
	}
	idx, exhausted := s.cache.GetAndAdvance(cacheKey, length)

	if exhausted {
		s.logger.Debug("data exhausted",
			zap.String("cacheKey", maskCacheKey(cacheKey)),
			zap.Int("index", idx),
			zap.Int("length", length),
		)
		return generated.GetClassicGexChain404JSONResponse{
			Error: ptr("No more data available"),
		}, nil
	}

	// Get data at index
	gexData, err := s.loader.GetAtIndex(ctx, ticker, pkg, category, idx)
	if err != nil {
		if errors.Is(err, data.ErrIndexOutOfBounds) {
			return generated.GetClassicGexChain404JSONResponse{
				Error: ptr("Index out of bounds"),
			}, nil
		}
		return generated.GetClassicGexChain404JSONResponse{
			Error: ptr(err.Error()),
		}, nil
	}

	s.logger.Debug("returning data",
		zap.String("cacheKey", maskCacheKey(cacheKey)),
		zap.Int("index", idx),
		zap.Int64("timestamp", gexData.Timestamp),
	)

	// Convert json.RawMessage to []interface{}
	var strikes []interface{}
	if gexData.Strikes != nil {
		if err := json.Unmarshal(gexData.Strikes, &strikes); err != nil {
			s.logger.Warn("failed to unmarshal strikes", zap.Error(err))
		}
	}

	var maxPriors []interface{}
	if gexData.MaxPriors != nil {
		if err := json.Unmarshal(gexData.MaxPriors, &maxPriors); err != nil {
			s.logger.Warn("failed to unmarshal max_priors", zap.Error(err))
		}
	}

	return generated.GetClassicGexChain200JSONResponse{
		Timestamp:         gexData.Timestamp,
		Ticker:            gexData.Ticker,
		MinDte:            &gexData.MinDTE,
		SecMinDte:         &gexData.SecMinDTE,
		Spot:              &gexData.Spot,
		ZeroGamma:         &gexData.ZeroGamma,
		MajorPosVol:       &gexData.MajorPosVol,
		MajorPosOi:        &gexData.MajorPosOI,
		MajorNegVol:       &gexData.MajorNegVol,
		MajorNegOi:        &gexData.MajorNegOI,
		Strikes:           &strikes,
		SumGexVol:         &gexData.SumGexVol,
		SumGexOi:          &gexData.SumGexOI,
		DeltaRiskReversal: &gexData.DeltaRiskReversal,
		MaxPriors:         &maxPriors,
	}, nil
}

// GetTickers implements generated.StrictServerInterface
func (s *Server) GetTickers(ctx context.Context, request generated.GetTickersRequestObject) (generated.GetTickersResponseObject, error) {
	keys := s.loader.GetLoadedKeys()

	// Extract unique tickers
	tickerSet := make(map[string]bool)
	for _, key := range keys {
		parts := strings.Split(key, "/")
		if len(parts) >= 1 {
			tickerSet[parts[0]] = true
		}
	}

	// Categorize tickers - initialize as empty slices (not nil) for consistent JSON
	stocks := []string{}
	indexes := []string{}
	futures := []string{}

	for ticker := range tickerSet {
		switch {
		case config.IndexTickers[ticker]:
			indexes = append(indexes, ticker)
		case config.FutureTickers[ticker]:
			futures = append(futures, ticker)
		default:
			stocks = append(stocks, ticker)
		}
	}

	// Sort for consistent output
	sort.Strings(stocks)
	sort.Strings(indexes)
	sort.Strings(futures)

	return generated.GetTickers200JSONResponse{
		Stocks:  &stocks,
		Indexes: &indexes,
		Futures: &futures,
	}, nil
}

// GetTickersQuant implements generated.StrictServerInterface. Mirrors the live
// /tickers/quant route: stocks and indexes only (no futures).
func (s *Server) GetTickersQuant(ctx context.Context, request generated.GetTickersQuantRequestObject) (generated.GetTickersQuantResponseObject, error) {
	tickerSet := make(map[string]bool)
	for _, key := range s.loader.GetLoadedKeys() {
		if parts := strings.Split(key, "/"); len(parts) >= 1 {
			tickerSet[parts[0]] = true
		}
	}

	stocks := []string{}
	indexes := []string{}
	for ticker := range tickerSet {
		switch {
		case config.IndexTickers[ticker]:
			indexes = append(indexes, ticker)
		case config.FutureTickers[ticker]:
			// futures are not exposed on the quant endpoint
		default:
			stocks = append(stocks, ticker)
		}
	}
	sort.Strings(stocks)
	sort.Strings(indexes)

	return generated.GetTickersQuant200JSONResponse{
		Stocks:  &stocks,
		Indexes: &indexes,
	}, nil
}

// GetPackageCategories implements generated.StrictServerInterface. Returns the
// category names a data package supports (mirrors the live /{package}/categories).
func (s *Server) GetPackageCategories(ctx context.Context, request generated.GetPackageCategoriesRequestObject) (generated.GetPackageCategoriesResponseObject, error) {
	cats, ok := config.ValidCategories[config.Package(request.Package)]
	if !ok {
		return generated.GetPackageCategories400JSONResponse{
			Error: ptr("Invalid package: " + string(request.Package)),
		}, nil
	}
	out := append([]string(nil), cats...)
	sort.Strings(out)
	return generated.GetPackageCategories200JSONResponse(out), nil
}

// GetHistEod implements generated.StrictServerInterface. Serves the most recent
// EOD report archive (zip) for a ticker — the same archive the daemon downloads.
func (s *Server) GetHistEod(ctx context.Context, request generated.GetHistEodRequestObject) (generated.GetHistEodResponseObject, error) {
	// Newest date that actually has THIS ticker's archive (the globally-latest
	// date may not include every ticker).
	date := eod.LatestTickerDate(s.config.DataDir, request.Ticker)
	if date == "" {
		return generated.GetHistEod404JSONResponse{Error: ptr("No EOD archive available for " + request.Ticker)}, nil
	}
	f, err := os.Open(eod.ArchivePath(s.config.DataDir, date, request.Ticker))
	if err != nil {
		return generated.GetHistEod404JSONResponse{Error: ptr("No EOD archive available for " + request.Ticker)}, nil
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return generated.GetHistEod404JSONResponse{Error: ptr(err.Error())}, nil
	}
	// The generated Visit closes Body (an *os.File is an io.ReadCloser).
	return generated.GetHistEod200ApplicationzipResponse{Body: f, ContentLength: info.Size()}, nil
}

// GetHistSnapshot implements generated.StrictServerInterface. Mirrors the live
// API's URL-indirection: it returns a URL to the snapshot rather than the data.
// The faker points at its own /download route for that date/ticker/category.
func (s *Server) GetHistSnapshot(ctx context.Context, request generated.GetHistSnapshotRequestObject) (generated.GetHistSnapshotResponseObject, error) {
	pkg := string(request.Package)
	// A snapshot exists only if we archived that date/ticker.
	if _, err := os.Stat(eod.ManifestPath(eod.ArchivePath(s.config.DataDir, request.Date, request.Ticker))); err != nil {
		return generated.GetHistSnapshot404JSONResponse{
			Error: ptr("No snapshot for " + request.Ticker + "/" + pkg + "/" + request.Category + " on " + request.Date),
		}, nil
	}
	var url string
	if pkg == "orderflow" {
		url = fmt.Sprintf("/download/%s/%s/orderflow", request.Date, request.Ticker)
	} else {
		url = fmt.Sprintf("/download/%s/%s/%s/%s", request.Date, request.Ticker, pkg, request.Category)
	}
	// Match the live contract: bare ?noredirect returns the JSON {url}; absence
	// produces a 302 redirect to that URL.
	if request.Params.Noredirect != nil {
		return generated.GetHistSnapshot200JSONResponse{Url: url}, nil
	}
	return generated.GetHistSnapshot302Response{
		Headers: generated.GetHistSnapshot302ResponseHeaders{Location: url},
	}, nil
}

// GetHealth implements generated.StrictServerInterface
func (s *Server) GetHealth(ctx context.Context, request generated.GetHealthRequestObject) (generated.GetHealthResponseObject, error) {
	status := "ok"
	dataMode := generated.HealthResponseDataMode(s.config.DataMode)
	cacheMode := generated.HealthResponseCacheMode(s.config.CacheMode)
	return generated.GetHealth200JSONResponse{
		Status:    &status,
		DataDate:  &s.config.DataDate,
		DataMode:  &dataMode,
		CacheMode: &cacheMode,
	}, nil
}

// ResetCache implements generated.StrictServerInterface
func (s *Server) ResetCache(ctx context.Context, request generated.ResetCacheRequestObject) (generated.ResetCacheResponseObject, error) {
	apiKey := ""
	if request.Params.Key != nil {
		apiKey = *request.Params.Key
	}

	count := s.cache.Reset(apiKey)

	status := "success"
	message := "All cache positions reset to index 0"
	if apiKey != "" {
		message = "Cache positions reset for key: " + maskAPIKey(apiKey)
	}

	s.logger.Info("cache reset",
		zap.String("apiKey", maskAPIKey(apiKey)),
		zap.Int("count", count),
	)

	return generated.ResetCache200JSONResponse{
		Status:  &status,
		Message: &message,
		Count:   &count,
	}, nil
}

// SeekToTimestamp implements generated.StrictServerInterface
func (s *Server) SeekToTimestamp(ctx context.Context, request generated.SeekToTimestampRequestObject) (generated.SeekToTimestampResponseObject, error) {
	targetTs := request.Body.Timestamp
	apiKey := request.Body.Key

	s.logger.Info("seek to timestamp request",
		zap.Int64("targetTimestamp", targetTs),
		zap.String("apiKey", maskAPIKey(apiKey)),
	)

	// Get all loaded data keys
	dataKeys := s.loader.GetLoadedKeys()

	// Range-aware resolver: ReloadableLoader implements RangeSeeker (cross-day resolution +
	// gap/clamp/end-policy in range mode; identical to FindIndexByTimestamp for a single day).
	seeker, _ := s.loader.(data.RangeSeeker)

	var details []generated.SeekPositionDetail
	var outOfRangeCount int
	positionsSet := 0
	var repRes data.SeekResult // representative resolution → top-level resolved_ts/day/in_gap/clamped
	haveRep := false

	for _, dataKey := range dataKeys {
		// Parse ticker/pkg/category from key (format: "ticker/pkg/category")
		ticker, pkg, category := parseDataKey(dataKey)
		if ticker == "" {
			continue
		}

		// Resolve the seek for this stream (range-aware when a span is loaded).
		var res data.SeekResult
		var err error
		if seeker != nil {
			res, err = seeker.SeekResolve(ctx, ticker, pkg, category, targetTs, s.config.RangeEndPolicy)
		} else {
			var idx int
			var ts int64
			idx, ts, err = s.loader.FindIndexByTimestamp(ctx, ticker, pkg, category, targetTs)
			res = data.SeekResult{Index: idx, ResolvedTs: ts, Clamped: "none"}
		}
		if err != nil {
			if errors.Is(err, data.ErrTimestampOutOfRange) {
				outOfRangeCount++
			}
			continue
		}

		// Build cache key and set position for REST API
		var cacheKey string
		if s.config.EndpointCacheMode == "shared" {
			cacheKey = data.SharedCacheKey(ticker, pkg, apiKey)
		} else {
			cacheKey = data.CacheKey(ticker, pkg, category, apiKey)
		}

		s.cache.SetIndex(cacheKey, res.Index)
		positionsSet++

		// Also update WebSocket positions for this data stream
		wsHubs := mapDataKeyToWSHubs(pkg, category)
		for _, hub := range wsHubs {
			wsCacheKey := data.WSCacheKey(hub, ticker, category, apiKey)
			s.cache.SetIndex(wsCacheKey, res.Index)
			positionsSet++
		}

		if !haveRep {
			repRes = res
			haveRep = true
		}
		dk, idx, ts := dataKey, res.Index, res.ResolvedTs
		details = append(details, generated.SeekPositionDetail{
			DataKey:   &dk,
			Index:     &idx,
			Timestamp: &ts,
		})
	}

	// If ALL streams returned timestamp out of range, return error
	if positionsSet == 0 && outOfRangeCount > 0 {
		s.logger.Debug("seek to timestamp: timestamp out of range",
			zap.Int64("targetTimestamp", targetTs),
			zap.Int("outOfRangeCount", outOfRangeCount),
		)
		return generated.SeekToTimestamp400JSONResponse{
			Error: ptr("Timestamp is after all available data"),
		}, nil
	}

	// If no data was found at all
	if positionsSet == 0 {
		return generated.SeekToTimestamp400JSONResponse{
			Error: ptr("No data found to seek"),
		}, nil
	}

	status := "success"
	message := fmt.Sprintf("Seeked %d positions to timestamp %d", positionsSet, targetTs)

	s.logger.Info("seek to timestamp complete",
		zap.Int64("targetTimestamp", targetTs),
		zap.String("apiKey", maskAPIKey(apiKey)),
		zap.Int("positionsSet", positionsSet),
	)

	resolvedTs := repRes.ResolvedTs
	day := repRes.Date
	if day == "" {
		day = s.reloadManager.CurrentDate() // single-day fallback → the loaded date
	}
	inGap := repRes.InGap
	clampVal := repRes.Clamped
	if clampVal == "" {
		clampVal = "none"
	}
	clamped := generated.SeekToTimestampResponseClamped(clampVal)
	reason := seekReason(repRes)

	return generated.SeekToTimestamp200JSONResponse{
		Status:       &status,
		Message:      &message,
		PositionsSet: &positionsSet,
		Details:      &details,
		ResolvedTs:   &resolvedTs,
		Day:          &day,
		InGap:        &inGap,
		Clamped:      &clamped,
		Reason:       &reason,
	}, nil
}

// seekReason gives a short human note for a clamped/gap resolution (empty on a clean in-session hit).
func seekReason(r data.SeekResult) string {
	switch {
	case r.InGap:
		return "between sessions — clamped to next open"
	case r.Clamped == "start":
		return "before loaded span — clamped to start"
	case r.Clamped == "end":
		return "after loaded span — clamped to end"
	default:
		return ""
	}
}

// parseDataKey splits a data key into ticker, pkg, and category
func parseDataKey(key string) (ticker, pkg, category string) {
	parts := strings.Split(key, "/")
	if len(parts) != 3 {
		return "", "", ""
	}
	return parts[0], parts[1], parts[2]
}

// mapDataKeyToWSHubs returns the WebSocket hub names for a given package/category.
// This maps REST data keys to their corresponding WebSocket cache key hubs.
func mapDataKeyToWSHubs(pkg, category string) []string {
	switch pkg {
	case "orderflow":
		return []string{"orderflow"}
	case "classic":
		if strings.HasPrefix(category, "gex_") {
			return []string{"classic"}
		}
	case "state":
		if strings.HasPrefix(category, "gex_") {
			return []string{"state_gex"}
		}
		// Greek categories
		switch category {
		case "delta_zero", "gamma_zero", "vanna_zero", "charm_zero":
			return []string{"state_greeks_zero"}
		case "delta_one", "gamma_one", "vanna_one", "charm_one":
			return []string{"state_greeks_one"}
		}
	}
	return nil
}

// Type classification helpers
var aggregationTypes = map[string]bool{"gex_full": true, "gex_zero": true, "gex_one": true}
var greekTypes = map[string]bool{
	"delta_zero": true, "gamma_zero": true, "delta_one": true, "gamma_one": true,
	"charm_zero": true, "vanna_zero": true, "charm_one": true, "vanna_one": true,
}

// GetStateProfile implements generated.StrictServerInterface
// Unified handler for both GEX profile (aggregations) and Greek profile (greeks)
func (s *Server) GetStateProfile(ctx context.Context, request generated.GetStateProfileRequestObject) (generated.GetStateProfileResponseObject, error) {
	ticker := request.Ticker
	typeParam := string(request.Type)
	apiKey := authKeyFromContext(ctx)
	pkg := "state"

	s.logger.Debug("state profile request",
		zap.String("ticker", ticker),
		zap.String("type", typeParam),
		zap.String("apiKey", maskAPIKey(apiKey)),
	)

	// Determine category based on type
	var category string
	isGreek := greekTypes[typeParam]
	if aggregationTypes[typeParam] {
		category = typeParam // path segment already carries the gex_ prefix (gex_full/gex_zero/gex_one)
	} else if isGreek {
		category = typeParam // delta_zero, gamma_zero, etc.
	} else {
		return generated.GetStateProfile400JSONResponse{
			Error: ptr("Invalid type parameter: " + typeParam),
		}, nil
	}

	// Check if data exists
	if !s.loader.Exists(ticker, pkg, category) {
		return generated.GetStateProfile404JSONResponse{
			Error: ptr("Data not found for " + ticker + "/state/" + typeParam),
		}, nil
	}

	// Get data length
	length, err := s.loader.GetLength(ticker, pkg, category)
	if err != nil {
		return generated.GetStateProfile404JSONResponse{
			Error: ptr(err.Error()),
		}, nil
	}

	// Build cache key based on endpoint cache mode
	var cacheKey string
	if s.config.EndpointCacheMode == "shared" {
		cacheKey = data.SharedCacheKey(ticker, pkg, apiKey)
	} else {
		// Independent mode - include category
		cacheKey = data.CacheKey(ticker, pkg, category, apiKey)
	}

	// Get index and check exhaustion
	idx, exhausted := s.cache.GetAndAdvance(cacheKey, length)

	if exhausted {
		s.logger.Debug("data exhausted",
			zap.String("cacheKey", maskCacheKey(cacheKey)),
			zap.Int("index", idx),
			zap.Int("length", length),
		)
		return generated.GetStateProfile404JSONResponse{
			Error: ptr("No more data available"),
		}, nil
	}

	// Get raw data at index
	rawData, err := s.loader.GetRawAtIndex(ctx, ticker, pkg, category, idx)
	if err != nil {
		if errors.Is(err, data.ErrIndexOutOfBounds) {
			return generated.GetStateProfile404JSONResponse{
				Error: ptr("Index out of bounds"),
			}, nil
		}
		return generated.GetStateProfile404JSONResponse{
			Error: ptr(err.Error()),
		}, nil
	}

	s.logger.Debug("returning state profile data",
		zap.String("cacheKey", maskCacheKey(cacheKey)),
		zap.Int("index", idx),
		zap.Bool("isGreek", isGreek),
	)

	// Return appropriate response based on type
	if isGreek {
		// Parse into GreekData and build GreekProfileData response
		var greekData data.GreekData
		if err := json.Unmarshal(rawData, &greekData); err != nil {
			s.logger.Error("failed to parse greek data", zap.Error(err))
			return generated.GetStateProfile404JSONResponse{
				Error: ptr("Failed to parse greek data"),
			}, nil
		}

		var miniContracts [][]interface{}
		if greekData.MiniContracts != nil {
			if err := json.Unmarshal(greekData.MiniContracts, &miniContracts); err != nil {
				s.logger.Warn("failed to unmarshal mini_contracts", zap.Error(err))
			}
		}

		return stateProfileGreekDataResponse{
			Timestamp:       greekData.Timestamp,
			Ticker:          greekData.Ticker,
			Spot:            &greekData.Spot,
			MinDte:          &greekData.MinDTE,
			SecMinDte:       &greekData.SecMinDTE,
			MajorPositive:   &greekData.MajorPositive,
			MajorNegative:   &greekData.MajorNegative,
			MajorLongGamma:  &greekData.MajorLongGamma,
			MajorShortGamma: &greekData.MajorShortGamma,
			MiniContracts:   &miniContracts,
		}, nil
	}

	// Parse into GexData and build GexData response
	var gexData data.GexData
	if err := json.Unmarshal(rawData, &gexData); err != nil {
		s.logger.Error("failed to parse gex data", zap.Error(err))
		return generated.GetStateProfile404JSONResponse{
			Error: ptr("Failed to parse gex data"),
		}, nil
	}

	var strikes []interface{}
	if gexData.Strikes != nil {
		if err := json.Unmarshal(gexData.Strikes, &strikes); err != nil {
			s.logger.Warn("failed to unmarshal strikes", zap.Error(err))
		}
	}

	var maxPriors []interface{}
	if gexData.MaxPriors != nil {
		if err := json.Unmarshal(gexData.MaxPriors, &maxPriors); err != nil {
			s.logger.Warn("failed to unmarshal max_priors", zap.Error(err))
		}
	}

	return stateProfileGexDataResponse{
		Timestamp:         gexData.Timestamp,
		Ticker:            gexData.Ticker,
		MinDte:            &gexData.MinDTE,
		SecMinDte:         &gexData.SecMinDTE,
		Spot:              &gexData.Spot,
		ZeroGamma:         &gexData.ZeroGamma,
		MajorPosVol:       &gexData.MajorPosVol,
		MajorPosOi:        &gexData.MajorPosOI,
		MajorNegVol:       &gexData.MajorNegVol,
		MajorNegOi:        &gexData.MajorNegOI,
		Strikes:           &strikes,
		SumGexVol:         &gexData.SumGexVol,
		SumGexOi:          &gexData.SumGexOI,
		DeltaRiskReversal: &gexData.DeltaRiskReversal,
		MaxPriors:         &maxPriors,
	}, nil
}

// GetStateGexMajors implements generated.StrictServerInterface
func (s *Server) GetStateGexMajors(ctx context.Context, request generated.GetStateGexMajorsRequestObject) (generated.GetStateGexMajorsResponseObject, error) {
	ticker := request.Ticker
	typeParam := string(request.Type)
	apiKey := authKeyFromContext(ctx)

	// Map type to internal category format
	category := typeParam // path segment already carries the gex_ prefix (gex_full/gex_zero/gex_one)
	pkg := "state"

	s.logger.Debug("state gex majors request",
		zap.String("ticker", ticker),
		zap.String("type", typeParam),
		zap.String("category", category),
		zap.String("apiKey", maskAPIKey(apiKey)),
	)

	// Check if data exists
	if !s.loader.Exists(ticker, pkg, category) {
		return generated.GetStateGexMajors404JSONResponse{
			Error: ptr("Data not found for " + ticker + "/state/" + typeParam),
		}, nil
	}

	// Get data length
	length, err := s.loader.GetLength(ticker, pkg, category)
	if err != nil {
		return generated.GetStateGexMajors404JSONResponse{
			Error: ptr(err.Error()),
		}, nil
	}

	// Build cache key based on endpoint cache mode
	var cacheKey string
	if s.config.EndpointCacheMode == "shared" {
		cacheKey = data.SharedCacheKey(ticker, pkg, apiKey)
	} else {
		// Independent mode - include category with _majors suffix
		cacheKey = data.CacheKey(ticker, pkg, category+"_majors", apiKey)
	}

	// Get index and check exhaustion
	idx, exhausted := s.cache.GetAndAdvance(cacheKey, length)

	if exhausted {
		s.logger.Debug("data exhausted",
			zap.String("cacheKey", maskCacheKey(cacheKey)),
			zap.Int("index", idx),
			zap.Int("length", length),
		)
		return generated.GetStateGexMajors404JSONResponse{
			Error: ptr("No more data available"),
		}, nil
	}

	// Get data at index
	gexData, err := s.loader.GetAtIndex(ctx, ticker, pkg, category, idx)
	if err != nil {
		if errors.Is(err, data.ErrIndexOutOfBounds) {
			return generated.GetStateGexMajors404JSONResponse{
				Error: ptr("Index out of bounds"),
			}, nil
		}
		return generated.GetStateGexMajors404JSONResponse{
			Error: ptr(err.Error()),
		}, nil
	}

	s.logger.Debug("returning state majors data",
		zap.String("cacheKey", maskCacheKey(cacheKey)),
		zap.Int("index", idx),
		zap.Int64("timestamp", gexData.Timestamp),
	)

	return generated.GetStateGexMajors200JSONResponse{
		Timestamp: gexData.Timestamp,
		Ticker:    gexData.Ticker,
		Spot:      &gexData.Spot,
		MposVol:   &gexData.MajorPosVol,
		MposOi:    &gexData.MajorPosOI,
		MnegVol:   &gexData.MajorNegVol,
		MnegOi:    &gexData.MajorNegOI,
		ZeroGamma: &gexData.ZeroGamma,
		NetGexVol: &gexData.SumGexVol,
		NetGexOi:  &gexData.SumGexOI,
	}, nil
}

// GetStateGexMaxChange implements generated.StrictServerInterface
func (s *Server) GetStateGexMaxChange(ctx context.Context, request generated.GetStateGexMaxChangeRequestObject) (generated.GetStateGexMaxChangeResponseObject, error) {
	ticker := request.Ticker
	typeParam := string(request.Type)
	apiKey := authKeyFromContext(ctx)

	// Map type to internal category format
	category := typeParam // path segment already carries the gex_ prefix (gex_full/gex_zero/gex_one)
	pkg := "state"

	s.logger.Debug("state gex max change request",
		zap.String("ticker", ticker),
		zap.String("type", typeParam),
		zap.String("category", category),
		zap.String("apiKey", maskAPIKey(apiKey)),
	)

	// Check if data exists
	if !s.loader.Exists(ticker, pkg, category) {
		return generated.GetStateGexMaxChange404JSONResponse{
			Error: ptr("Data not found for " + ticker + "/state/" + typeParam),
		}, nil
	}

	// Get data length
	length, err := s.loader.GetLength(ticker, pkg, category)
	if err != nil {
		return generated.GetStateGexMaxChange404JSONResponse{
			Error: ptr(err.Error()),
		}, nil
	}

	// Build cache key based on endpoint cache mode
	var cacheKey string
	if s.config.EndpointCacheMode == "shared" {
		cacheKey = data.SharedCacheKey(ticker, pkg, apiKey)
	} else {
		// Independent mode - include category with _maxchange suffix
		cacheKey = data.CacheKey(ticker, pkg, category+"_maxchange", apiKey)
	}

	// Get index and check exhaustion
	idx, exhausted := s.cache.GetAndAdvance(cacheKey, length)

	if exhausted {
		s.logger.Debug("data exhausted",
			zap.String("cacheKey", maskCacheKey(cacheKey)),
			zap.Int("index", idx),
			zap.Int("length", length),
		)
		return generated.GetStateGexMaxChange404JSONResponse{
			Error: ptr("No more data available"),
		}, nil
	}

	// Get data at index
	gexData, err := s.loader.GetAtIndex(ctx, ticker, pkg, category, idx)
	if err != nil {
		if errors.Is(err, data.ErrIndexOutOfBounds) {
			return generated.GetStateGexMaxChange404JSONResponse{
				Error: ptr("Index out of bounds"),
			}, nil
		}
		return generated.GetStateGexMaxChange404JSONResponse{
			Error: ptr(err.Error()),
		}, nil
	}

	// Parse max_priors: [[strike, gex], [strike, gex], ...] (6 pairs)
	var maxPriors [][]float32
	if gexData.MaxPriors != nil {
		if err := json.Unmarshal(gexData.MaxPriors, &maxPriors); err != nil {
			s.logger.Warn("failed to unmarshal max_priors", zap.Error(err))
		}
	}

	s.logger.Debug("returning state max change data",
		zap.String("cacheKey", maskCacheKey(cacheKey)),
		zap.Int("index", idx),
		zap.Int64("timestamp", gexData.Timestamp),
	)

	// Map to response fields (ensure we have 6 elements)
	response := generated.GetStateGexMaxChange200JSONResponse{
		Timestamp: gexData.Timestamp,
		Ticker:    gexData.Ticker,
	}

	if len(maxPriors) >= 6 {
		response.Current = &maxPriors[0]
		response.One = &maxPriors[1]
		response.Five = &maxPriors[2]
		response.Ten = &maxPriors[3]
		response.Fifteen = &maxPriors[4]
		response.Thirty = &maxPriors[5]
	}

	return response, nil
}

// GetOrderflowLatest implements generated.StrictServerInterface
func (s *Server) GetOrderflowLatest(ctx context.Context, request generated.GetOrderflowLatestRequestObject) (generated.GetOrderflowLatestResponseObject, error) {
	ticker := request.Ticker
	apiKey := authKeyFromContext(ctx)
	pkg := "orderflow"
	category := "orderflow"

	s.logger.Debug("orderflow latest request",
		zap.String("ticker", ticker),
		zap.String("apiKey", maskAPIKey(apiKey)),
	)

	// Check if data exists
	if !s.loader.Exists(ticker, pkg, category) {
		return generated.GetOrderflowLatest404JSONResponse{
			Error: ptr("Data not found for " + ticker + "/orderflow/orderflow"),
		}, nil
	}

	// Get data length
	length, err := s.loader.GetLength(ticker, pkg, category)
	if err != nil {
		return generated.GetOrderflowLatest404JSONResponse{
			Error: ptr(err.Error()),
		}, nil
	}

	// Build cache key based on endpoint cache mode
	var cacheKey string
	if s.config.EndpointCacheMode == "shared" {
		cacheKey = data.SharedCacheKey(ticker, pkg, apiKey)
	} else {
		cacheKey = data.CacheKey(ticker, pkg, category, apiKey)
	}

	idx, exhausted := s.cache.GetAndAdvance(cacheKey, length)

	if exhausted {
		s.logger.Debug("data exhausted",
			zap.String("cacheKey", maskCacheKey(cacheKey)),
			zap.Int("index", idx),
			zap.Int("length", length),
		)
		return generated.GetOrderflowLatest404JSONResponse{
			Error: ptr("No more data available"),
		}, nil
	}

	// Get raw data and parse
	rawData, err := s.loader.GetRawAtIndex(ctx, ticker, pkg, category, idx)
	if err != nil {
		if errors.Is(err, data.ErrIndexOutOfBounds) {
			return generated.GetOrderflowLatest404JSONResponse{
				Error: ptr("Index out of bounds"),
			}, nil
		}
		return generated.GetOrderflowLatest404JSONResponse{
			Error: ptr(err.Error()),
		}, nil
	}

	var ofData data.OrderflowData
	if err := json.Unmarshal(rawData, &ofData); err != nil {
		s.logger.Error("failed to parse orderflow data", zap.Error(err))
		return generated.GetOrderflowLatest404JSONResponse{
			Error: ptr("Failed to parse orderflow data"),
		}, nil
	}

	s.logger.Debug("returning orderflow data",
		zap.String("cacheKey", maskCacheKey(cacheKey)),
		zap.Int("index", idx),
		zap.Int64("timestamp", ofData.Timestamp),
	)

	return generated.GetOrderflowLatest200JSONResponse{
		Timestamp:     ofData.Timestamp,
		Ticker:        ofData.Ticker,
		Spot:          &ofData.Spot,
		ZMlgamma:      f32ptr(ofData.ZMlgamma),
		ZMsgamma:      f32ptr(ofData.ZMsgamma),
		OMlgamma:      f32ptr(ofData.OMlgamma),
		OMsgamma:      f32ptr(ofData.OMsgamma),
		ZeroMcall:     f32ptr(ofData.ZeroMcall),
		ZeroMput:      f32ptr(ofData.ZeroMput),
		OneMcall:      f32ptr(ofData.OneMcall),
		OneMput:       f32ptr(ofData.OneMput),
		Zcvr:          f32ptr(ofData.Zcvr),
		Ocvr:          f32ptr(ofData.Ocvr),
		Zgr:           f32ptr(ofData.Zgr),
		Ogr:           f32ptr(ofData.Ogr),
		Zvanna:        f32ptr(ofData.Zvanna),
		Ovanna:        f32ptr(ofData.Ovanna),
		Zcharm:        f32ptr(ofData.Zcharm),
		Ocharm:        f32ptr(ofData.Ocharm),
		AggDex:        f32ptr(ofData.AggDex),
		OneAggDex:     f32ptr(ofData.OneAggDex),
		AggCallDex:    f32ptr(ofData.AggCallDex),
		OneAggCallDex: f32ptr(ofData.OneAggCallDex),
		AggPutDex:     f32ptr(ofData.AggPutDex),
		OneAggPutDex:  f32ptr(ofData.OneAggPutDex),
		NetDex:        f32ptr(ofData.NetDex),
		OneNetDex:     f32ptr(ofData.OneNetDex),
		NetCallDex:    f32ptr(ofData.NetCallDex),
		OneNetCallDex: f32ptr(ofData.OneNetCallDex),
		NetPutDex:     f32ptr(ofData.NetPutDex),
		OneNetPutDex:  f32ptr(ofData.OneNetPutDex),
		Dexoflow:      f32ptr(ofData.Dexoflow),
		Gexoflow:      f32ptr(ofData.Gexoflow),
		Cvroflow:      f32ptr(ofData.Cvroflow),
		OneDexoflow:   f32ptr(ofData.OneDexoflow),
		OneGexoflow:   f32ptr(ofData.OneGexoflow),
		OneCvroflow:   f32ptr(ofData.OneCvroflow),
	}, nil
}

func ptr[T any](v T) *T { return &v }

// f32ptr converts float64 to *float32 for OpenAPI response fields
func f32ptr(v float64) *float32 {
	f := float32(v)
	return &f
}

func maskAPIKey(string) string { return "[REDACTED]" }

// maskCacheKey masks the API key portion of a cache key (format: ticker/pkg/category/apiKey)
func maskCacheKey(cacheKey string) string {
	parts := strings.Split(cacheKey, "/")
	if len(parts) >= 4 {
		parts[len(parts)-1] = maskAPIKey(parts[len(parts)-1])
		return strings.Join(parts, "/")
	}
	return cacheKey
}

// GetAvailableDates implements generated.StrictServerInterface
func (s *Server) GetAvailableDates(ctx context.Context, request generated.GetAvailableDatesRequestObject) (generated.GetAvailableDatesResponseObject, error) {
	entries, err := os.ReadDir(s.config.DataDir)
	if err != nil {
		s.logger.Error("failed to read data directory", zap.Error(err))
		return nil, err
	}

	datePattern := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	var dates []string

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if name == ".staging" {
			continue
		}
		if datePattern.MatchString(name) {
			dates = append(dates, name)
		}
	}

	sort.Strings(dates)
	count := len(dates)

	s.logger.Debug("available dates request",
		zap.Int("count", count),
		zap.Strings("dates", dates),
	)

	return generated.GetAvailableDates200JSONResponse{
		Dates: &dates,
		Count: &count,
	}, nil
}

// GetCurrentDate implements generated.StrictServerInterface
func (s *Server) GetCurrentDate(ctx context.Context, request generated.GetCurrentDateRequestObject) (generated.GetCurrentDateResponseObject, error) {
	filesLoaded := len(s.loader.GetLoadedKeys())

	s.logger.Debug("current date request",
		zap.String("currentDate", s.config.DataDate),
		zap.Time("loadedAt", s.loadedAt),
		zap.Int("filesLoaded", filesLoaded),
	)

	return generated.GetCurrentDate200JSONResponse{
		CurrentDate: &s.config.DataDate,
		LoadedAt:    &s.loadedAt,
		FilesLoaded: &filesLoaded,
	}, nil
}

// GetAvailableData implements generated.StrictServerInterface
func (s *Server) GetAvailableData(ctx context.Context, request generated.GetAvailableDataRequestObject) (generated.GetAvailableDataResponseObject, error) {
	date := request.Date
	if !isValidDateFormat(date) {
		emptyTickers := []generated.TickerData{}
		total := 0
		return generated.GetAvailableData200JSONResponse{
			Date:    &date,
			Tickers: &emptyTickers,
			Summary: &generated.DataSummary{TotalTickers: &total, TotalFiles: &total},
		}, nil
	}
	if _, err := os.Stat(filepath.Join(s.config.DataDir, date)); os.IsNotExist(err) {
		_ = eod.MaterializeDate(s.config.DataDir, date, s.logger)
	}
	tickerFilter := ""
	if request.Params.Ticker != nil {
		tickerFilter = *request.Params.Ticker
	}

	s.logger.Debug("available data request",
		zap.String("date", date),
		zap.String("tickerFilter", tickerFilter),
	)

	// Build path to date directory
	datePath := filepath.Join(s.config.DataDir, date)

	// Check if date directory exists
	if _, err := os.Stat(datePath); os.IsNotExist(err) {
		// Return empty response for non-existent date
		emptyTickers := []generated.TickerData{}
		totalTickers := 0
		totalFiles := 0
		return generated.GetAvailableData200JSONResponse{
			Date:    &date,
			Tickers: &emptyTickers,
			Summary: &generated.DataSummary{
				TotalTickers: &totalTickers,
				TotalFiles:   &totalFiles,
			},
		}, nil
	}

	// Scan for tickers
	tickerEntries, err := os.ReadDir(datePath)
	if err != nil {
		s.logger.Error("failed to read date directory", zap.Error(err))
		return nil, err
	}

	tickers := []generated.TickerData{}
	totalFiles := 0

	for _, tickerEntry := range tickerEntries {
		if !tickerEntry.IsDir() {
			continue
		}
		tickerName := tickerEntry.Name()

		// Apply ticker filter if specified
		if tickerFilter != "" && tickerName != tickerFilter {
			continue
		}

		tickerPath := filepath.Join(datePath, tickerName)
		pkgEntries, err := os.ReadDir(tickerPath)
		if err != nil {
			s.logger.Warn("failed to read ticker directory", zap.String("ticker", tickerName), zap.Error(err))
			continue
		}

		var packages []generated.PackageData

		for _, pkgEntry := range pkgEntries {
			if !pkgEntry.IsDir() {
				continue
			}
			pkgName := pkgEntry.Name()

			// Only include known packages
			var packageName generated.PackageDataName
			switch pkgName {
			case "classic":
				packageName = generated.PackageDataNameClassic
			case "state":
				packageName = generated.PackageDataNameState
			case "orderflow":
				packageName = generated.PackageDataNameOrderflow
			default:
				continue
			}

			pkgPath := filepath.Join(tickerPath, pkgName)
			categoryEntries, err := os.ReadDir(pkgPath)
			if err != nil {
				s.logger.Warn("failed to read package directory", zap.String("package", pkgName), zap.Error(err))
				continue
			}

			var categories []string
			for _, catEntry := range categoryEntries {
				if catEntry.IsDir() {
					continue
				}
				fileName := catEntry.Name()
				if strings.HasSuffix(fileName, ".jsonl") {
					category := strings.TrimSuffix(fileName, ".jsonl")
					categories = append(categories, category)
					totalFiles++
				}
			}

			if len(categories) > 0 {
				sort.Strings(categories)
				packages = append(packages, generated.PackageData{
					Name:       &packageName,
					Categories: &categories,
				})
			}
		}

		if len(packages) > 0 {
			tickers = append(tickers, generated.TickerData{
				Symbol:   &tickerName,
				Packages: &packages,
			})
		}
	}

	// Sort tickers alphabetically
	sort.Slice(tickers, func(i, j int) bool {
		return *tickers[i].Symbol < *tickers[j].Symbol
	})

	totalTickers := len(tickers)

	s.logger.Debug("available data response",
		zap.String("date", date),
		zap.Int("totalTickers", totalTickers),
		zap.Int("totalFiles", totalFiles),
	)

	return generated.GetAvailableData200JSONResponse{
		Date:    &date,
		Tickers: &tickers,
		Summary: &generated.DataSummary{
			TotalTickers: &totalTickers,
			TotalFiles:   &totalFiles,
		},
	}, nil
}

// downloadFileResponse implements file streaming for download endpoints
type downloadFileResponse struct {
	filePath string
	filename string
}

func (r *downloadFileResponse) serveFile(w http.ResponseWriter) error {
	file, err := os.Open(r.filePath)
	if err != nil {
		http.Error(w, "Failed to open file", http.StatusInternalServerError)
		return err
	}
	defer func() { _ = file.Close() }()

	stat, err := file.Stat()
	if err != nil {
		http.Error(w, "Failed to stat file", http.StatusInternalServerError)
		return err
	}

	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, r.filename))
	w.Header().Set("Content-Length", strconv.FormatInt(stat.Size(), 10))
	w.WriteHeader(http.StatusOK)

	_, err = io.Copy(w, file)
	return err
}

// classicDownloadResponse wraps downloadFileResponse for classic GEX downloads
type classicDownloadResponse struct {
	downloadFileResponse
}

func (r *classicDownloadResponse) VisitDownloadClassicGexResponse(w http.ResponseWriter) error {
	return r.serveFile(w)
}

// DownloadClassicGex implements generated.StrictServerInterface
func (s *Server) DownloadClassicGex(ctx context.Context, request generated.DownloadClassicGexRequestObject) (generated.DownloadClassicGexResponseObject, error) {
	date := request.Date
	ticker := request.Ticker
	aggregation := string(request.Aggregation)
	_ = eod.MaterializeTicker(s.config.DataDir, date, ticker, s.logger)

	// Construct file path: {DataDir}/{date}/{ticker}/classic/gex_{aggregation}.jsonl
	category := aggregation
	filePath := filepath.Join(s.config.DataDir, date, ticker, "classic", category+".jsonl")

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		s.logger.Warn("download file not found",
			zap.String("date", date),
			zap.String("ticker", ticker),
			zap.String("aggregation", aggregation),
			zap.String("filePath", filePath),
		)
		return generated.DownloadClassicGex404JSONResponse{
			Error: ptr(fmt.Sprintf("File not found: %s/%s/classic/%s.jsonl", date, ticker, category)),
		}, nil
	}

	filename := fmt.Sprintf("%s_%s_classic_%s.jsonl", date, ticker, category)

	s.logger.Info("download classic request",
		zap.String("date", date),
		zap.String("ticker", ticker),
		zap.String("aggregation", aggregation),
	)

	return &classicDownloadResponse{
		downloadFileResponse: downloadFileResponse{filePath: filePath, filename: filename},
	}, nil
}

// stateDownloadResponse wraps downloadFileResponse for state data downloads
type stateDownloadResponse struct {
	downloadFileResponse
}

func (r *stateDownloadResponse) VisitDownloadStateDataResponse(w http.ResponseWriter) error {
	return r.serveFile(w)
}

// DownloadStateData implements generated.StrictServerInterface
func (s *Server) DownloadStateData(ctx context.Context, request generated.DownloadStateDataRequestObject) (generated.DownloadStateDataResponseObject, error) {
	date := request.Date
	ticker := request.Ticker
	typeParam := string(request.Type)
	_ = eod.MaterializeTicker(s.config.DataDir, date, ticker, s.logger)

	// Determine category based on type (same logic as GetStateProfile)
	var category string
	if aggregationTypes[typeParam] {
		category = typeParam
	} else if greekTypes[typeParam] {
		category = typeParam
	} else {
		return generated.DownloadStateData404JSONResponse{
			Error: ptr("Invalid type parameter: " + typeParam),
		}, nil
	}

	// Construct file path: {DataDir}/{date}/{ticker}/state/{category}.jsonl
	filePath := filepath.Join(s.config.DataDir, date, ticker, "state", category+".jsonl")

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		s.logger.Warn("download file not found",
			zap.String("date", date),
			zap.String("ticker", ticker),
			zap.String("type", typeParam),
			zap.String("filePath", filePath),
		)
		return generated.DownloadStateData404JSONResponse{
			Error: ptr(fmt.Sprintf("File not found: %s/%s/state/%s.jsonl", date, ticker, category)),
		}, nil
	}

	filename := fmt.Sprintf("%s_%s_state_%s.jsonl", date, ticker, category)

	s.logger.Info("download state request",
		zap.String("date", date),
		zap.String("ticker", ticker),
		zap.String("type", typeParam),
	)

	return &stateDownloadResponse{
		downloadFileResponse: downloadFileResponse{filePath: filePath, filename: filename},
	}, nil
}

// orderflowDownloadResponse wraps downloadFileResponse for orderflow data downloads
type orderflowDownloadResponse struct {
	downloadFileResponse
}

func (r *orderflowDownloadResponse) VisitDownloadOrderflowResponse(w http.ResponseWriter) error {
	return r.serveFile(w)
}

// DownloadOrderflow implements generated.StrictServerInterface
func (s *Server) DownloadOrderflow(ctx context.Context, request generated.DownloadOrderflowRequestObject) (generated.DownloadOrderflowResponseObject, error) {
	date := request.Date
	ticker := request.Ticker
	_ = eod.MaterializeTicker(s.config.DataDir, date, ticker, s.logger)

	// Construct file path: {DataDir}/{date}/{ticker}/orderflow/orderflow.jsonl
	filePath := filepath.Join(s.config.DataDir, date, ticker, "orderflow", "orderflow.jsonl")

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		s.logger.Warn("download file not found",
			zap.String("date", date),
			zap.String("ticker", ticker),
			zap.String("filePath", filePath),
		)
		return generated.DownloadOrderflow404JSONResponse{
			Error: ptr(fmt.Sprintf("File not found: %s/%s/orderflow/orderflow.jsonl", date, ticker)),
		}, nil
	}

	filename := fmt.Sprintf("%s_%s_orderflow.jsonl", date, ticker)

	s.logger.Info("download orderflow request",
		zap.String("date", date),
		zap.String("ticker", ticker),
	)

	return &orderflowDownloadResponse{
		downloadFileResponse: downloadFileResponse{filePath: filePath, filename: filename},
	}, nil
}

// categoryToPathParam maps file categories to URL path parameters
func categoryToPathParam(pkg, category string) string {
	if pkg == "classic" || pkg == "state" {
		if strings.HasPrefix(category, "gex_") {
			return strings.TrimPrefix(category, "gex_")
		}
	}
	return category
}

// buildDownloadPath constructs the download URL path for a given package/category
func buildDownloadPath(date, ticker, pkg, category string) string {
	switch pkg {
	case "classic":
		return fmt.Sprintf("/download/%s/%s/classic/%s", date, ticker, categoryToPathParam(pkg, category))
	case "state":
		return fmt.Sprintf("/download/%s/%s/state/%s", date, ticker, categoryToPathParam(pkg, category))
	case "orderflow":
		return fmt.Sprintf("/download/%s/%s/orderflow", date, ticker)
	}
	return ""
}

// GetDownloadLinks implements generated.StrictServerInterface
func (s *Server) GetDownloadLinks(ctx context.Context, request generated.GetDownloadLinksRequestObject) (generated.GetDownloadLinksResponseObject, error) {
	date := request.Date
	ticker := request.Ticker
	_ = eod.MaterializeTicker(s.config.DataDir, date, ticker, s.logger)

	s.logger.Debug("download links request",
		zap.String("date", date),
		zap.String("ticker", ticker),
	)

	// Build path to ticker directory
	tickerPath := filepath.Join(s.config.DataDir, date, ticker)

	// Check if ticker directory exists
	if _, err := os.Stat(tickerPath); os.IsNotExist(err) {
		return generated.GetDownloadLinks404JSONResponse{
			Error: ptr(fmt.Sprintf("No data found for %s/%s", date, ticker)),
		}, nil
	}

	// Scan for packages
	pkgEntries, err := os.ReadDir(tickerPath)
	if err != nil {
		s.logger.Error("failed to read ticker directory", zap.Error(err))
		return nil, err
	}

	links := make(map[string][]string)
	totalLinks := 0

	for _, pkgEntry := range pkgEntries {
		if !pkgEntry.IsDir() {
			continue
		}
		pkgName := pkgEntry.Name()

		// Only process known packages
		if pkgName != "classic" && pkgName != "state" && pkgName != "orderflow" {
			continue
		}

		pkgPath := filepath.Join(tickerPath, pkgName)
		catEntries, err := os.ReadDir(pkgPath)
		if err != nil {
			s.logger.Warn("failed to read package directory",
				zap.String("package", pkgName),
				zap.Error(err),
			)
			continue
		}

		var pkgLinks []string
		for _, catEntry := range catEntries {
			if catEntry.IsDir() {
				continue
			}
			fileName := catEntry.Name()
			if !strings.HasSuffix(fileName, ".jsonl") {
				continue
			}

			category := strings.TrimSuffix(fileName, ".jsonl")
			path := buildDownloadPath(date, ticker, pkgName, category)

			pkgLinks = append(pkgLinks, path)
			totalLinks++
		}

		if len(pkgLinks) > 0 {
			sort.Strings(pkgLinks)
			links[pkgName] = pkgLinks
		}
	}

	if totalLinks == 0 {
		return generated.GetDownloadLinks404JSONResponse{
			Error: ptr(fmt.Sprintf("No data files found for %s/%s", date, ticker)),
		}, nil
	}

	s.logger.Debug("download links response",
		zap.String("date", date),
		zap.String("ticker", ticker),
		zap.Int("totalLinks", totalLinks),
	)

	return generated.GetDownloadLinks200JSONResponse{
		Date:   date,
		Ticker: ticker,
		Links:  links,
		Summary: &generated.DownloadLinksSummary{
			TotalLinks: &totalLinks,
		},
	}, nil
}

// ReloadDate implements generated.StrictServerInterface
func (s *Server) ReloadDate(ctx context.Context, request generated.ReloadDateRequestObject) (generated.ReloadDateResponseObject, error) {
	newDate := request.Body.Date

	s.logger.Info("reload date request",
		zap.String("currentDate", s.config.DataDate),
		zap.String("newDate", newDate),
	)

	// Check if reload manager is available
	if s.reloadManager == nil {
		return generated.ReloadDate500JSONResponse{
			Error: ptr("Reload not available: server not configured for hot reload"),
		}, nil
	}

	// Perform the reload
	result, err := s.reloadManager.Reload(ctx, newDate)
	if err != nil {
		errMsg := err.Error()

		// Check for specific error types
		if strings.Contains(errMsg, "already in progress") {
			return generated.ReloadDate409JSONResponse{
				Error: ptr(errMsg),
			}, nil
		}

		if strings.Contains(errMsg, "not found") || strings.Contains(errMsg, "invalid date format") {
			return generated.ReloadDate400JSONResponse{
				Error: ptr(errMsg),
			}, nil
		}

		return generated.ReloadDate500JSONResponse{
			Error: ptr(errMsg),
		}, nil
	}

	// Update server's loadedAt time
	s.loadedAt = result.LoadedAt

	status := "success"

	s.logger.Info("reload date complete",
		zap.String("previousDate", result.PreviousDate),
		zap.String("newDate", result.NewDate),
		zap.Time("loadedAt", result.LoadedAt),
		zap.Int("filesLoaded", result.FilesLoaded),
	)

	return generated.ReloadDate200JSONResponse{
		Status:       &status,
		PreviousDate: &result.PreviousDate,
		NewDate:      &result.NewDate,
		LoadedAt:     &result.LoadedAt,
		FilesLoaded:  &result.FilesLoaded,
	}, nil
}
