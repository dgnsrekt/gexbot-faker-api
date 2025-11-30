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
	loader   data.DataLoader
	cache    *data.IndexCache
	config   *config.ServerConfig
	logger   *zap.Logger
	loadedAt time.Time
}

func NewServer(loader data.DataLoader, cache *data.IndexCache, cfg *config.ServerConfig, logger *zap.Logger) *Server {
	return &Server{
		loader:   loader,
		cache:    cache,
		config:   cfg,
		logger:   logger,
		loadedAt: time.Now(),
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

// Type classification helpers
var aggregationTypes = map[string]bool{"full": true, "zero": true, "one": true}
var greekTypes = map[string]bool{
	"delta_zero": true, "gamma_zero": true, "delta_one": true, "gamma_one": true,
	"charm_zero": true, "vanna_zero": true, "charm_one": true, "vanna_one": true,
}

// GetStateProfile implements generated.StrictServerInterface
// Unified handler for both GEX profile (aggregations) and Greek profile (greeks)
func (s *Server) GetStateProfile(ctx context.Context, request generated.GetStateProfileRequestObject) (generated.GetStateProfileResponseObject, error) {
	ticker := request.Ticker
	typeParam := string(request.Type)
	apiKey := request.Params.Key
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
		category = "gex_" + typeParam // full→gex_full, zero→gex_zero, one→gex_one
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
	apiKey := request.Params.Key

	// Map type to internal category format
	category := "gex_" + typeParam // full→gex_full, zero→gex_zero, one→gex_one
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
	apiKey := request.Params.Key

	// Map type to internal category format
	category := "gex_" + typeParam // full→gex_full, zero→gex_zero, one→gex_one
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
	apiKey := request.Params.Key
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
	filesLoaded := 12 // 6 tickers × 2 packages (classic + state)

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
	defer file.Close()

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
	ticker := request.Ticker
	aggregation := string(request.Aggregation)

	// Construct file path: {DataDir}/{DataDate}/{ticker}/classic/gex_{aggregation}.jsonl
	category := "gex_" + aggregation
	filePath := filepath.Join(s.config.DataDir, s.config.DataDate, ticker, "classic", category+".jsonl")

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		s.logger.Warn("download file not found",
			zap.String("ticker", ticker),
			zap.String("aggregation", aggregation),
			zap.String("filePath", filePath),
		)
		return generated.DownloadClassicGex404JSONResponse{
			Error: ptr(fmt.Sprintf("File not found: %s/classic/%s.jsonl", ticker, category)),
		}, nil
	}

	filename := fmt.Sprintf("%s_classic_%s.jsonl", ticker, category)

	s.logger.Info("download classic request",
		zap.String("ticker", ticker),
		zap.String("aggregation", aggregation),
		zap.String("apiKey", maskAPIKey(request.Params.Key)),
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
	ticker := request.Ticker
	typeParam := string(request.Type)

	// Determine category based on type (same logic as GetStateProfile)
	var category string
	if aggregationTypes[typeParam] {
		category = "gex_" + typeParam
	} else if greekTypes[typeParam] {
		category = typeParam
	} else {
		return generated.DownloadStateData404JSONResponse{
			Error: ptr("Invalid type parameter: " + typeParam),
		}, nil
	}

	// Construct file path: {DataDir}/{DataDate}/{ticker}/state/{category}.jsonl
	filePath := filepath.Join(s.config.DataDir, s.config.DataDate, ticker, "state", category+".jsonl")

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		s.logger.Warn("download file not found",
			zap.String("ticker", ticker),
			zap.String("type", typeParam),
			zap.String("filePath", filePath),
		)
		return generated.DownloadStateData404JSONResponse{
			Error: ptr(fmt.Sprintf("File not found: %s/state/%s.jsonl", ticker, category)),
		}, nil
	}

	filename := fmt.Sprintf("%s_state_%s.jsonl", ticker, category)

	s.logger.Info("download state request",
		zap.String("ticker", ticker),
		zap.String("type", typeParam),
		zap.String("apiKey", maskAPIKey(request.Params.Key)),
	)

	return &stateDownloadResponse{
		downloadFileResponse: downloadFileResponse{filePath: filePath, filename: filename},
	}, nil
}
