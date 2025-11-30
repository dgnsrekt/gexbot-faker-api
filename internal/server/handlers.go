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
		zap.String("apiKey", apiKey),
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
			zap.String("cacheKey", cacheKey),
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
		zap.String("cacheKey", cacheKey),
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
		zap.String("apiKey", apiKey),
		zap.Int("count", count),
	)

	return generated.ResetCache200JSONResponse{
		Status:  &status,
		Message: &message,
		Count:   &count,
	}, nil
}

func ptr[T any](v T) *T { return &v }
