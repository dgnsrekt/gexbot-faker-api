package server

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"go.uber.org/zap"

	"github.com/dgnsrekt/gexbot-downloader/internal/api/generated"
	"github.com/dgnsrekt/gexbot-downloader/internal/config"
	"github.com/dgnsrekt/gexbot-downloader/internal/data"
)

type Server struct {
	loader *data.MemoryLoader
	cache  *data.IndexCache
	config *config.ServerConfig
	logger *zap.Logger
}

func NewServer(loader *data.MemoryLoader, cache *data.IndexCache, cfg *config.ServerConfig, logger *zap.Logger) *Server {
	return &Server{
		loader: loader,
		cache:  cache,
		config: cfg,
		logger: logger,
	}
}

// Compile-time interface verification
var _ generated.StrictServerInterface = (*Server)(nil)

// GetGexData implements generated.StrictServerInterface
func (s *Server) GetGexData(ctx context.Context, request generated.GetGexDataRequestObject) (generated.GetGexDataResponseObject, error) {
	ticker := request.Ticker
	pkg := request.Package
	category := request.Category
	apiKey := request.Params.Key

	s.logger.Debug("gex data request",
		zap.String("ticker", ticker),
		zap.String("package", string(pkg)),
		zap.String("category", string(category)),
		zap.String("apiKey", apiKey),
	)

	// Check if data exists
	if !s.loader.Exists(ticker, string(pkg), string(category)) {
		return generated.GetGexData404JSONResponse{
			Error: ptr("Data not found for " + ticker + "/" + string(pkg) + "/" + string(category)),
		}, nil
	}

	// Get data length
	length, err := s.loader.GetLength(ticker, string(pkg), string(category))
	if err != nil {
		return generated.GetGexData404JSONResponse{
			Error: ptr(err.Error()),
		}, nil
	}

	// Get index and check exhaustion
	cacheKey := data.CacheKey(ticker, string(pkg), string(category), apiKey)
	idx, exhausted := s.cache.GetAndAdvance(cacheKey, length)

	if exhausted {
		s.logger.Debug("data exhausted",
			zap.String("cacheKey", cacheKey),
			zap.Int("index", idx),
			zap.Int("length", length),
		)
		return generated.GetGexData404JSONResponse{
			Error: ptr("No more data available"),
		}, nil
	}

	// Get data at index
	gexData, err := s.loader.GetAtIndex(ctx, ticker, string(pkg), string(category), idx)
	if err != nil {
		if errors.Is(err, data.ErrIndexOutOfBounds) {
			return generated.GetGexData404JSONResponse{
				Error: ptr("Index out of bounds"),
			}, nil
		}
		return generated.GetGexData404JSONResponse{
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

	return generated.GetGexData200JSONResponse{
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

	tickers := make([]generated.TickerInfo, 0, len(tickerSet))
	for ticker := range tickerSet {
		tickerType := generated.Stock
		if ticker == "SPX" || ticker == "NDX" || ticker == "RUT" || ticker == "VIX" {
			tickerType = generated.Index
		}
		tickers = append(tickers, generated.TickerInfo{
			Symbol: &ticker,
			Type:   &tickerType,
		})
	}

	count := len(tickers)
	return generated.GetTickers200JSONResponse{
		Tickers: &tickers,
		Count:   &count,
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
