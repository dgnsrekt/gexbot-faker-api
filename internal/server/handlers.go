package server

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"

	"go.uber.org/zap"

	"github.com/dgnsrekt/gexbot-downloader/internal/api/generated"
	"github.com/dgnsrekt/gexbot-downloader/internal/config"
	"github.com/dgnsrekt/gexbot-downloader/internal/data"
)

type Server struct {
	loader data.DataLoader
	cache  *data.IndexCache
	config *config.ServerConfig
	logger *zap.Logger
}

func NewServer(loader data.DataLoader, cache *data.IndexCache, cfg *config.ServerConfig, logger *zap.Logger) *Server {
	return &Server{
		loader: loader,
		cache:  cache,
		config: cfg,
		logger: logger,
	}
}

// Compile-time interface verification
var _ generated.StrictServerInterface = (*Server)(nil)

// GetClassicGexMajors implements generated.StrictServerInterface
func (s *Server) GetClassicGexMajors(ctx context.Context, request generated.GetClassicGexMajorsRequestObject) (generated.GetClassicGexMajorsResponseObject, error) {
	ticker := request.Ticker
	aggregation := string(request.Aggregation)
	apiKey := request.Params.Key

	// Map aggregation to internal category format
	category := "gex_" + aggregation // full→gex_full, zero→gex_zero, one→gex_one
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

	// Build cache key - append _majors suffix in independent mode
	cacheCategory := category
	if s.config.EndpointCacheMode == "independent" {
		cacheCategory += "_majors"
	}
	cacheKey := data.CacheKey(ticker, pkg, cacheCategory, apiKey)
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
	apiKey := request.Params.Key

	// Map aggregation to internal category format
	category := "gex_" + aggregation // full→gex_full, zero→gex_zero, one→gex_one
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

	// Build cache key - append _maxchange suffix in independent mode
	cacheCategory := category
	if s.config.EndpointCacheMode == "independent" {
		cacheCategory += "_maxchange"
	}
	cacheKey := data.CacheKey(ticker, pkg, cacheCategory, apiKey)
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
	apiKey := request.Params.Key

	// Map aggregation to internal category format
	category := "gex_" + aggregation // full→gex_full, zero→gex_zero, one→gex_one
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

	// Get index and check exhaustion
	cacheKey := data.CacheKey(ticker, pkg, category, apiKey)
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
	knownIndexes := map[string]bool{"SPX": true, "VIX": true, "NDX": true, "RUT": true}

	for ticker := range tickerSet {
		switch {
		case knownIndexes[ticker]:
			indexes = append(indexes, ticker)
		case strings.Contains(ticker, "_"):
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
		message = "Cache positions reset for key: " + apiKey
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

// GetStateGexProfile implements generated.StrictServerInterface
func (s *Server) GetStateGexProfile(ctx context.Context, request generated.GetStateGexProfileRequestObject) (generated.GetStateGexProfileResponseObject, error) {
	ticker := request.Ticker
	aggregation := string(request.Aggregation)
	apiKey := request.Params.Key

	// Map aggregation to internal category format
	category := "gex_" + aggregation // full→gex_full, zero→gex_zero, one→gex_one
	pkg := "state"

	s.logger.Debug("state gex profile request",
		zap.String("ticker", ticker),
		zap.String("aggregation", aggregation),
		zap.String("category", category),
		zap.String("apiKey", maskAPIKey(apiKey)),
	)

	// Check if data exists
	if !s.loader.Exists(ticker, pkg, category) {
		return generated.GetStateGexProfile404JSONResponse{
			Error: ptr("Data not found for " + ticker + "/state/" + aggregation),
		}, nil
	}

	// Get data length
	length, err := s.loader.GetLength(ticker, pkg, category)
	if err != nil {
		return generated.GetStateGexProfile404JSONResponse{
			Error: ptr(err.Error()),
		}, nil
	}

	// Get index and check exhaustion
	cacheKey := data.CacheKey(ticker, pkg, category, apiKey)
	idx, exhausted := s.cache.GetAndAdvance(cacheKey, length)

	if exhausted {
		s.logger.Debug("data exhausted",
			zap.String("cacheKey", maskCacheKey(cacheKey)),
			zap.Int("index", idx),
			zap.Int("length", length),
		)
		return generated.GetStateGexProfile404JSONResponse{
			Error: ptr("No more data available"),
		}, nil
	}

	// Get data at index
	gexData, err := s.loader.GetAtIndex(ctx, ticker, pkg, category, idx)
	if err != nil {
		if errors.Is(err, data.ErrIndexOutOfBounds) {
			return generated.GetStateGexProfile404JSONResponse{
				Error: ptr("Index out of bounds"),
			}, nil
		}
		return generated.GetStateGexProfile404JSONResponse{
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

	return generated.GetStateGexProfile200JSONResponse{
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
	aggregation := string(request.Aggregation)
	apiKey := request.Params.Key

	// Map aggregation to internal category format
	category := "gex_" + aggregation // full→gex_full, zero→gex_zero, one→gex_one
	pkg := "state"

	s.logger.Debug("state gex majors request",
		zap.String("ticker", ticker),
		zap.String("aggregation", aggregation),
		zap.String("category", category),
		zap.String("apiKey", maskAPIKey(apiKey)),
	)

	// Check if data exists
	if !s.loader.Exists(ticker, pkg, category) {
		return generated.GetStateGexMajors404JSONResponse{
			Error: ptr("Data not found for " + ticker + "/state/" + aggregation),
		}, nil
	}

	// Get data length
	length, err := s.loader.GetLength(ticker, pkg, category)
	if err != nil {
		return generated.GetStateGexMajors404JSONResponse{
			Error: ptr(err.Error()),
		}, nil
	}

	// Build cache key - append _majors suffix in independent mode
	cacheCategory := category
	if s.config.EndpointCacheMode == "independent" {
		cacheCategory += "_majors"
	}
	cacheKey := data.CacheKey(ticker, pkg, cacheCategory, apiKey)
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

func ptr[T any](v T) *T { return &v }

// maskAPIKey returns a masked version of the API key showing only first 4 chars
func maskAPIKey(key string) string {
	if len(key) <= 4 {
		return key
	}
	return key[:4] + "****"
}

// maskCacheKey masks the API key portion of a cache key (format: ticker/pkg/category/apiKey)
func maskCacheKey(cacheKey string) string {
	parts := strings.Split(cacheKey, "/")
	if len(parts) >= 4 {
		parts[len(parts)-1] = maskAPIKey(parts[len(parts)-1])
		return strings.Join(parts, "/")
	}
	return cacheKey
}
