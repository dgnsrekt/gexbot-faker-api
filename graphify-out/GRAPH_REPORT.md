# Graph Report - .  (2026-08-09)

## Corpus Check
- 105 files · ~91,326 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 1451 nodes · 2869 edges · 92 communities (57 shown, 35 thin omitted)
- Extraction: 89% EXTRACTED · 11% INFERRED · 0% AMBIGUOUS · INFERRED: 319 edges (avg confidence: 0.8)
- Token cost: 0 input · 0 output

## Community Hubs (Navigation)
- Daemon Entrypoint
- Generated API Types (1)
- WebSocket Hubs & Protobuf (1)
- Configuration (1)
- Generated API Types (2)
- Generated API Types (3)
- EOD Archive Pipeline
- HTTP Handlers & Routing (1)
- WebSocket Hubs & Protobuf (2)
- WebSocket Hubs & Protobuf (3)
- WebSocket Hubs & Protobuf (4)
- Data Loading & Cache (1)
- Sync Broadcaster
- Root (1)
- Generated API Types (4)
- Generated API Types (5)
- HTTP Handlers & Routing (2)
- Archived Plans
- Download Manager
- Generated API Types (6)
- HTTP Handlers & Routing (3)
- README
- WebSocket Hubs & Protobuf (5)
- API Layer (1)
- internal
- Compatibility Audit (1)
- WebSocket Hubs & Protobuf (6)
- WebSocket Hubs & Protobuf (7)
- WebSocket Hubs & Protobuf (8)
- WebSocket Hubs & Protobuf (9)
- HTTP Handlers & Routing (4)
- Compatibility Audit (2)
- Data Loading & Cache (2)
- Generated API Types (7)
- Generated API Types (8)
- WebSocket Hubs & Protobuf (10)
- WebSocket Hubs & Protobuf (11)
- Data Loading & Cache (3)
- WebSocket Hubs & Protobuf (12)
- WebSocket Hubs & Protobuf (13)
- WebSocket Hubs & Protobuf (14)
- WebSocket Hubs & Protobuf (15)
- WebSocket Hubs & Protobuf (16)
- WebSocket Hubs & Protobuf (17)
- WebSocket Hubs & Protobuf (18)
- WebSocket Hubs & Protobuf (19)
- WebSocket Hubs & Protobuf (20)
- WebSocket Hubs & Protobuf (21)
- WebSocket Hubs & Protobuf (22)
- WebSocket Hubs & Protobuf (23)
- WebSocket Hubs & Protobuf (24)
- Generated API Types (9)
- WebSocket Hubs & Protobuf (25)
- WebSocket Hubs & Protobuf (26)
- WebSocket Hubs & Protobuf (27)
- Generated API Types (10)
- Generated API Types (11)
- HTTP Handlers & Routing (5)
- WebSocket Hubs & Protobuf (28)
- WebSocket Hubs & Protobuf (29)
- WebSocket Hubs & Protobuf (30)
- WebSocket Hubs & Protobuf (31)
- WebSocket Hubs & Protobuf (32)
- Data Loading & Cache (4)
- Generated API Types (12)
- Generated API Types (13)
- Generated API Types (14)
- Generated API Types (15)
- Generated API Types (16)
- WebSocket Hubs & Protobuf (33)
- Generated API Types (17)
- Generated API Types (18)
- Generated API Types (19)
- Generated API Types (20)
- Configuration (2)
- Generated API Types (21)
- Generated API Types (22)
- Generated API Types (23)
- Configuration (3)
- WebSocket Hubs & Protobuf (34)
- API Layer (2)
- CLAUDE
- .golangci.yml
- Root (2)
- go.mod

## God Nodes (most connected - your core abstractions)
1. `NewEncoder()` - 68 edges
2. `Orderflow` - 46 edges
3. `Handler()` - 36 edges
4. `Server` - 31 edges
5. `ServerInterfaceWrapper` - 30 edges
6. `strictHandler` - 29 edges
7. `Unimplemented` - 26 edges
8. `IndexCache` - 26 edges
9. `Gex` - 26 edges
10. `SyncBroadcaster` - 24 edges

## Surprising Connections (you probably didn't know these)
- `Five-Hub WebSocket Architecture` --semantically_similar_to--> `Five WebSocket Channels`  [INFERRED] [semantically similar]
  CLAUDE.md → api/asyncapi.yaml
- `WebSocket Streaming Protocol` --semantically_similar_to--> `GEX Faker WebSocket API`  [INFERRED] [semantically similar]
  WEBSOCKET.md → api/asyncapi.yaml
- `Wire-Compatible Orderflow Streaming` --semantically_similar_to--> `JSON-Protobuf-Zstd Encoding Pipeline`  [INFERRED] [semantically similar]
  docs/archived/PLAN_0003_30-11-2025_sun_websocket-orderflow-hub.md → WEBSOCKET.md
- `WebSocket Orderflow Hub Implementation Summary` --semantically_similar_to--> `GEX Faker WebSocket API`  [INFERRED] [semantically similar]
  docs/archived/SUMMARY_PLAN_0003_30-11-2025_sun_websocket-orderflow-hub.md → api/asyncapi.yaml
- `executeEODDownload()` --calls--> `ArchivePath()`  [INFERRED]
  cmd/daemon/download.go → internal/eod/archive.go

## Import Cycles
- None detected.

## Hyperedges (group relationships)
- **EOD Report Canonical Archive Pipeline** — readme_eod_archive, readme_jsonl_data, readme_daemon_service, docs_gexbot_live_compatibility_audit_eod_report [INFERRED 0.85]
- **WebSocket Wire Protocol Flow** — websocket_azure_web_pubsub_protocol, websocket_encoding_pipeline, api_asyncapi_message_schemas, docs_archived_plan_0003_30_11_2025_sun_websocket_orderflow_hub_wire_compatible_streaming [INFERRED 0.95]
- **2026-08 endpoint compatibility findings** — compatibility_audit_2026_08_09_auth_parity, compatibility_audit_2026_08_09_gex_path_parity, compatibility_audit_2026_08_09_response_body_compat, compatibility_audit_2026_08_09_coverage_gaps [EXTRACTED 1.00]
- **Compatibility audit evidence chain** — compatibility_readme, compatibility_upstream_openapi, docs_gexbot_live_compatibility_audit [EXTRACTED 0.85]
- **docker-compose service topology** — docker_compose_gex_faker_api, docker_compose_gex_daemon, docker_compose_gex_tools [EXTRACTED 1.00]
- **GexBot Live Parity Remediation** — docs_gexbot_live_compatibility_audit_bearer_auth, docs_gexbot_live_compatibility_audit_negotiate_lifecycle, docs_gexbot_live_compatibility_audit_eod_report, docs_gexbot_live_compatibility_audit_futures_conversion [INFERRED 0.75]
- **Historical Data Download and Replay Lifecycle** — docs_archived_research_0001_29_11_2025_sat_quant_historical_analysis_historical_api_contract, docs_archived_plan_0001_29_11_2025_sat_gexbot_historical_downloader_atomic_download_pipeline, readme_historical_downloader, claude_data_loading [INFERRED 0.85]

## Communities (92 total, 35 thin omitted)

### Community 0 - "Daemon Entrypoint"
Cohesion: 0.05
Nodes (52): Calendar, getEnvBoolOrDefault(), getEnvIntOrDefault(), getEnvOrDefault(), LoadDaemonConfig(), T, TestDefaultSchedule(), convertFile() (+44 more)

### Community 1 - "Generated API Types (1)"
Cohesion: 0.03
Nodes (31): GetClassicGexChain200JSONResponse, GetClassicGexChain400JSONResponse, GetClassicGexChain401JSONResponse, GetClassicGexMajors200JSONResponse, GetClassicGexMajors400JSONResponse, GetClassicGexMajors401JSONResponse, GetClassicGexMaxChange200JSONResponse, GetClassicGexMaxChange400JSONResponse (+23 more)

### Community 2 - "WebSocket Hubs & Protobuf (1)"
Cohesion: 0.07
Nodes (40): Client, HistoryResponse, HTTPClient, main(), run(), Context, Duration, Logger (+32 more)

### Community 3 - "Configuration (1)"
Cohesion: 0.06
Nodes (42): convertCmd(), convertFile(), convertJSONToJSONL(), Command, downloadCmd(), Command, filterMarketDays(), generateTasks() (+34 more)

### Community 4 - "Generated API Types (2)"
Cohesion: 0.08
Nodes (11): GetCurrentDate200JSONResponse, GetDownloadLinks404JSONResponse, GetFuturesConversion400JSONResponse, GetHealth200JSONResponse, GetOrderflowLatest404JSONResponse, GetTickers200JSONResponse, ResetCache200JSONResponse, ResetCacheParams (+3 more)

### Community 5 - "Generated API Types (3)"
Cohesion: 0.05
Nodes (45): AvailableDataResponse, AvailableDatesResponse, CurrentDateResponse, DataSummary, DownloadClassicGex200ApplicationxNdjsonResponse, DownloadLinksResponse, DownloadLinksSummary, DownloadOrderflow200ApplicationxNdjsonResponse (+37 more)

### Community 6 - "EOD Archive Pipeline"
Cohesion: 0.12
Nodes (41): eodArchiveDates(), eodCmd(), eodDates(), eodMaterializeCmd(), eodPackCmd(), eodPruneCmd(), eodTickers(), eodVerifyCmd() (+33 more)

### Community 7 - "HTTP Handlers & Routing (1)"
Cohesion: 0.11
Nodes (35): GetOptionsExpiriesRequestObject, GetOptionsExpiriesResponseObject, easterSunday(), generateExpiries(), Context, Month, Server, Time (+27 more)

### Community 8 - "WebSocket Hubs & Protobuf (2)"
Cohesion: 0.06
Nodes (10): MiniContract, MiniContractPriors, OptionProfile, file_option_profile_proto_init(), file_option_profile_proto_rawDescGZIP(), Message, MessageState, SizeCache (+2 more)

### Community 9 - "WebSocket Hubs & Protobuf (3)"
Cohesion: 0.05
Nodes (4): MessageState, SizeCache, UnknownFields, Orderflow

### Community 10 - "WebSocket Hubs & Protobuf (4)"
Cohesion: 0.07
Nodes (23): Conn, Hub, Logger, Request, ResponseWriter, RWMutex, Hub, buildAckMessage() (+15 more)

### Community 11 - "Data Loading & Cache (1)"
Cohesion: 0.10
Nodes (15): MemoryLoader, StreamLoader, File, DataKey(), extractTimestamp(), Context, GexData, Logger (+7 more)

### Community 12 - "Sync Broadcaster"
Cohesion: 0.13
Nodes (16): Flusher, Context, Duration, Logger, Request, ResponseWriter, RWMutex, maskAPIKey() (+8 more)

### Community 13 - "Root (1)"
Cohesion: 0.10
Nodes (23): Bool, fatal(), main(), normalize(), T, TestNormalizeSortsAndIsDeterministic(), ServerConfig, detectLatestDate() (+15 more)

### Community 14 - "Generated API Types (4)"
Cohesion: 0.10
Nodes (15): ChiServerOptions, GetAvailableData200JSONResponse, GetAvailableDataParams, GetAvailableDates200JSONResponse, GetOptionsExpiries404JSONResponse, GetTickersQuant200JSONResponse, MiddlewareFunc, ReloadDate500JSONResponse (+7 more)

### Community 15 - "Generated API Types (5)"
Cohesion: 0.09
Nodes (20): DownloadOrderflowRequestObject, GetAvailableDataRequestObject, GetAvailableDataResponseObject, GetAvailableDatesRequestObject, GetAvailableDatesResponseObject, GetCurrentDateRequestObject, GetCurrentDateResponseObject, GetHistEodResponseObject (+12 more)

### Community 16 - "HTTP Handlers & Routing (2)"
Cohesion: 0.15
Nodes (24): GetFuturesConversionParams, GetFuturesConversionParamsFuture, GetFuturesConversionParamsModel, GetFuturesConversionParamsTicker, GetFuturesConversionRequestObject, GetFuturesConversionResponseObject, clTermination(), conversionParams() (+16 more)

### Community 17 - "Archived Plans"
Cohesion: 0.08
Nodes (26): Web PubSub Message Schemas, GEX Faker WebSocket API, Five WebSocket Channels, JSONL Data Loading and Playback, Five-Hub WebSocket Architecture, Default Downloader Configuration, GexBot Tickers, Packages, and Categories, Concurrent Atomic Download Pipeline (+18 more)

### Community 18 - "Download Manager"
Cohesion: 0.13
Nodes (14): Manager, mockClient, Task, TaskResult, Client, Context, Logger, NewManager() (+6 more)

### Community 19 - "Generated API Types (6)"
Cohesion: 0.16
Nodes (18): GetClassicGexChainResponseObject, GetClassicGexMajorsResponseObject, GetClassicGexMaxChangeRequestObject, GetClassicGexMaxChangeResponseObject, GetOrderflowLatestRequestObject, GetOrderflowLatestResponseObject, GetStateGexMajorsRequestObject, GetStateGexMajorsResponseObject (+10 more)

### Community 20 - "HTTP Handlers & Routing (3)"
Cohesion: 0.13
Nodes (24): GetSwagger(), T, PathToRawSpec(), asyncapiHandler(), asyncapiUIHandler(), authMiddleware(), corsMiddleware(), Context (+16 more)

### Community 21 - "README"
Cohesion: 0.13
Nodes (24): GexBot Live Compatibility Audit, Bearer Auth and Required Headers, go test ./compatibility check, Drop-in Replacement Goal, Endpoint Compatibility Matrix, Live EOD Report Endpoint, Faker Control-Plane Extensions, Futures Conversion Endpoint (+16 more)

### Community 22 - "WebSocket Hubs & Protobuf (5)"
Cohesion: 0.09
Nodes (5): Any, isMessageData_Data, MessageData, MessageData_ProtobufData, UpstreamMessage_EventMessage_

### Community 23 - "API Layer (1)"
Cohesion: 0.10
Nodes (21): getClassicGexChain endpoint, getFuturesConversion endpoint, getHistEod endpoint, getHistSnapshot endpoint, getOptionsExpiries endpoint, getOrderflowLatest endpoint, getPackageCategories endpoint, getStateProfile endpoint (+13 more)

### Community 24 - "internal"
Cohesion: 0.12
Nodes (10): Client, Context, NewManager(), Context, T, Writer, TestStagingManager(), Downloader (+2 more)

### Community 25 - "Compatibility Audit (1)"
Cohesion: 0.23
Nodes (18): contains(), T, operationKey(), operationKeys(), readJSON(), readYAML(), schemaForOperation(), TestCompatibilityClassificationsMatchFakerSpec() (+10 more)

### Community 27 - "WebSocket Hubs & Protobuf (7)"
Cohesion: 0.14
Nodes (6): file_webpubsub_messages_proto_init(), init(), DownstreamMessage_SystemMessage_, isDownstreamMessage_SystemMessage_Message, MessageData_BinaryData, MessageData_TextData

### Community 28 - "WebSocket Hubs & Protobuf (8)"
Cohesion: 0.13
Nodes (3): SizeCache, DownstreamMessage_AckMessage_, DownstreamMessage_AckMessage_ErrorMessage

### Community 29 - "WebSocket Hubs & Protobuf (9)"
Cohesion: 0.13
Nodes (4): UnknownFields, DownstreamMessage, DownstreamMessage_PongMessage_, isDownstreamMessage_Message

### Community 30 - "HTTP Handlers & Routing (4)"
Cohesion: 0.23
Nodes (9): buildDownloadPath(), categoryToPathParam(), ResponseWriter, classicDownloadResponse, downloadFileResponse, orderflowDownloadResponse, stateDownloadResponse, stateProfileGexDataResponse (+1 more)

### Community 31 - "Compatibility Audit (2)"
Cohesion: 0.15
Nodes (15): GexBot compatibility evidence README, Upstream GexBot OpenAPI contract, GET /{package}/categories, GET /{ticker}/classic/{category}, GET /options/{ticker}/expiries, GET /futures/conversion, GET /hist/{ticker}/{package}/{category}/{date}, GET /hist/eod/{ticker} (+7 more)

### Community 32 - "Data Loading & Cache (2)"
Cohesion: 0.18
Nodes (6): DataLoader, ReloadableLoader, Context, GexData, RWMutex, NewReloadableLoader()

### Community 33 - "Generated API Types (7)"
Cohesion: 0.16
Nodes (9): GetHistEod404JSONResponse, GetStateGexMaxChange404JSONResponse, GetStateGexMaxChangeParamsType, strictHandler, StrictHTTPServerOptions, StrictServerInterface, NewStrictHandler(), NewStrictHandlerWithOptions() (+1 more)

### Community 34 - "Generated API Types (8)"
Cohesion: 0.14
Nodes (10): DownloadClassicGexRequestObject, DownloadClassicGexResponseObject, DownloadStateDataRequestObject, DownloadStateDataResponseObject, GetDownloadLinksRequestObject, GetDownloadLinksResponseObject, GetPackageCategories200JSONResponse, GetPackageCategoriesResponseObject (+2 more)

### Community 35 - "WebSocket Hubs & Protobuf (10)"
Cohesion: 0.18
Nodes (4): MaxPriors, MaxPriorsTuple, SizeCache, UnknownFields

### Community 36 - "WebSocket Hubs & Protobuf (11)"
Cohesion: 0.18
Nodes (4): MessageState, isUpstreamMessage_Message, UpstreamMessage, UpstreamMessage_PingMessage_

### Community 37 - "Data Loading & Cache (3)"
Cohesion: 0.23
Nodes (5): CacheMode, IndexCache, RWMutex, NewIndexCache(), WSCacheKey()

### Community 38 - "WebSocket Hubs & Protobuf (12)"
Cohesion: 0.35
Nodes (6): apiKeyFromAuthHeader(), Hub, Logger, Request, ResponseWriter, NegotiateHandler

### Community 39 - "WebSocket Hubs & Protobuf (13)"
Cohesion: 0.36
Nodes (8): extractTicker(), Context, Duration, Hub, Logger, NewStreamer(), ReloadChecker, Streamer

### Community 40 - "WebSocket Hubs & Protobuf (14)"
Cohesion: 0.38
Nodes (7): extractClassicTickerAndCategory(), Context, Duration, Hub, Logger, NewClassicStreamer(), ClassicStreamer

### Community 41 - "WebSocket Hubs & Protobuf (15)"
Cohesion: 0.38
Nodes (7): extractGexTickerAndCategory(), Context, Duration, Hub, Logger, NewGexStreamer(), GexStreamer

### Community 42 - "WebSocket Hubs & Protobuf (16)"
Cohesion: 0.38
Nodes (7): extractGreekOneTickerAndCategory(), Context, Duration, Hub, Logger, NewGreekOneStreamer(), GreekOneStreamer

### Community 43 - "WebSocket Hubs & Protobuf (17)"
Cohesion: 0.38
Nodes (7): extractGreekTickerAndCategory(), Context, Duration, Hub, Logger, NewGreekStreamer(), GreekStreamer

### Community 51 - "Generated API Types (9)"
Cohesion: 0.32
Nodes (4): GetHistSnapshot404JSONResponse, GetHistSnapshotParams, GetHistSnapshotParamsPackage, GetHistSnapshotRequestObject

### Community 55 - "Generated API Types (10)"
Cohesion: 0.40
Nodes (5): GetHealthRequestObject, GetHealthResponseObject, HealthResponse, HealthResponseCacheMode, HealthResponseDataMode

### Community 56 - "Generated API Types (11)"
Cohesion: 0.33
Nodes (3): GetPackageCategories400JSONResponse, GetPackageCategoriesParamsPackage, GetPackageCategoriesRequestObject

### Community 57 - "HTTP Handlers & Routing (5)"
Cohesion: 0.33
Nodes (5): SeekToTimestampRequestObject, SeekToTimestampResponseObject, mapDataKeyToWSHubs(), parseDataKey(), SeekToTimestampJSONRequestBody

### Community 62 - "WebSocket Hubs & Protobuf (32)"
Cohesion: 0.33
Nodes (5): maskAPIKey(), NegotiatePatchRequest, NegotiatePatchResponse, NegotiatePostResponse, NegotiateResponse

### Community 63 - "Data Loading & Cache (4)"
Cohesion: 0.50
Nodes (4): GexData, GreekData, OrderflowData, RawMessage

### Community 69 - "WebSocket Hubs & Protobuf (33)"
Cohesion: 0.50
Nodes (3): file_orderflow_proto_init(), file_orderflow_proto_rawDescGZIP(), init()

## Knowledge Gaps
- **72 isolated node(s):** `matrixEntry`, `liveProbeDoc`, `github.com/dgnsrekt/gexbot-downloader`, `HistoryResponse`, `AvailableDatesResponse` (+67 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **35 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `run()` connect `WebSocket Hubs & Protobuf (1)` to `Data Loading & Cache (2)`, `Data Loading & Cache (3)`, `EOD Archive Pipeline`, `WebSocket Hubs & Protobuf (13)`, `WebSocket Hubs & Protobuf (14)`, `WebSocket Hubs & Protobuf (15)`, `WebSocket Hubs & Protobuf (16)`, `Data Loading & Cache (1)`, `Sync Broadcaster`, `Root (1)`, `WebSocket Hubs & Protobuf (17)`, `HTTP Handlers & Routing (3)`?**
  _High betweenness centrality (0.090) - this node is a cross-community bridge._
- **Why does `NewEncoder()` connect `Generated API Types (1)` to `WebSocket Hubs & Protobuf (1)`, `Generated API Types (2)`, `Generated API Types (4)`, `HTTP Handlers & Routing (4)`, `Generated API Types (7)`, `WebSocket Hubs & Protobuf (12)`, `WebSocket Hubs & Protobuf (13)`, `WebSocket Hubs & Protobuf (14)`, `WebSocket Hubs & Protobuf (15)`, `WebSocket Hubs & Protobuf (16)`, `WebSocket Hubs & Protobuf (17)`, `Generated API Types (9)`, `Generated API Types (11)`, `WebSocket Hubs & Protobuf (30)`, `Generated API Types (12)`, `Generated API Types (13)`, `Generated API Types (14)`, `Generated API Types (15)`, `Generated API Types (16)`, `Generated API Types (17)`, `Generated API Types (18)`, `Generated API Types (19)`, `Generated API Types (20)`?**
  _High betweenness centrality (0.083) - this node is a cross-community bridge._
- **Why does `NewRouter()` connect `HTTP Handlers & Routing (3)` to `Generated API Types (7)`, `WebSocket Hubs & Protobuf (1)`, `WebSocket Hubs & Protobuf (12)`, `Sync Broadcaster`, `Generated API Types (4)`?**
  _High betweenness centrality (0.060) - this node is a cross-community bridge._
- **Are the 66 inferred relationships involving `NewEncoder()` (e.g. with `.VisitDownloadClassicGexResponse()` and `.VisitDownloadOrderflowResponse()`) actually correct?**
  _`NewEncoder()` has 66 INFERRED edges - model-reasoned connections that need verification._
- **What connects `matrixEntry`, `liveProbeDoc`, `github.com/dgnsrekt/gexbot-downloader` to the rest of the system?**
  _72 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `Daemon Entrypoint` be split into smaller, more focused modules?**
  _Cohesion score 0.05477477477477478 - nodes in this community are weakly interconnected._
- **Should `Generated API Types (1)` be split into smaller, more focused modules?**
  _Cohesion score 0.03278688524590164 - nodes in this community are weakly interconnected._