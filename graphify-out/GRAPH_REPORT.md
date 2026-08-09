# Graph Report - .  (2026-08-09)

## Corpus Check
- 123 files · ~94,997 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 1527 nodes · 3026 edges · 111 communities (72 shown, 39 thin omitted)
- Extraction: 88% EXTRACTED · 12% INFERRED · 0% AMBIGUOUS · INFERRED: 375 edges (avg confidence: 0.8)
- Token cost: 0 input · 59,919 output

## Community Hubs (Navigation)
- Generated API Types (1)
- WebSocket Hubs & Protobuf (1)
- Generated API Types (2)
- Configuration (1)
- Daemon Entrypoint
- Generated API Types (3)
- EOD Archive Pipeline
- WebSocket Hubs & Protobuf (2)
- WebSocket Hubs & Protobuf (3)
- HTTP Handlers & Routing (1)
- HTTP Handlers & Routing (2)
- Generated API Types (4)
- Notifications
- Data Loading & Cache (1)
- Sync Broadcaster
- HTTP Handlers & Routing (3)
- Archived Plans
- Download Manager
- HTTP Handlers & Routing (4)
- Staging
- Compatibility Audit (1)
- HTTP Handlers & Routing (5)
- WebSocket Hubs & Protobuf (4)
- WebSocket Hubs & Protobuf (5)
- Observability Stack (1)
- WebSocket Hubs & Protobuf (6)
- WebSocket Hubs & Protobuf (7)
- WebSocket Hubs & Protobuf (8)
- WebSocket Hubs & Protobuf (9)
- Compatibility Audit (2)
- Data Loading & Cache (2)
- HTTP Handlers & Routing (6)
- docs
- OBSERVABILITY
- WebSocket Hubs & Protobuf (10)
- WebSocket Hubs & Protobuf (11)
- WebSocket Hubs & Protobuf (12)
- WebSocket Hubs & Protobuf (13)
- Ticker Sync CLI
- Observability Metrics (1)
- WebSocket Hubs & Protobuf (14)
- Compatibility Audit (3)
- Data Loading & Cache (3)
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
- Generated API Types (5)
- WebSocket Hubs & Protobuf (25)
- WebSocket Hubs & Protobuf (26)
- Configuration (2)
- Observability Stack (2)
- Generated API Types (6)
- WebSocket Hubs & Protobuf (27)
- WebSocket Hubs & Protobuf (28)
- API Layer (1)
- WebSocket Hubs & Protobuf (29)
- WebSocket Hubs & Protobuf (30)
- README
- Generated API Types (7)
- Generated API Types (8)
- WebSocket Hubs & Protobuf (31)
- WebSocket Hubs & Protobuf (32)
- WebSocket Hubs & Protobuf (34)
- Data Loading & Cache (4)
- Generated API Types (9)
- WebSocket Hubs & Protobuf (35)
- Generated API Types (10)
- Generated API Types (11)
- Configuration (3)
- Generated API Types (12)
- Generated API Types (13)
- Generated API Types (14)
- Generated API Types (15)
- Generated API Types (16)
- Generated API Types (17)
- Generated API Types (18)
- Generated API Types (19)
- Generated API Types (20)
- Generated API Types (21)
- Generated API Types (22)
- Generated API Types (23)
- Configuration (4)
- WebSocket Hubs & Protobuf (36)
- API Layer (2)
- CLAUDE
- Generated API Types (24)
- Generated API Types (25)
- Generated API Types (26)
- Generated API Types (29)
- .golangci.yml
- Root
- Observability Stack (3)
- go.mod

## God Nodes (most connected - your core abstractions)
1. `NewEncoder()` - 69 edges
2. `Orderflow` - 46 edges
3. `Handler()` - 37 edges
4. `Server` - 31 edges
5. `ServerInterfaceWrapper` - 30 edges
6. `strictHandler` - 29 edges
7. `Unimplemented` - 26 edges
8. `IndexCache` - 26 edges
9. `Gex` - 26 edges
10. `SyncBroadcaster` - 24 edges

## Surprising Connections (you probably didn't know these)
- `runCleanup()` --calls--> `LatestDate()`  [INFERRED]
  cmd/daemon/main.go → internal/eod/archive.go
- `Five-Hub WebSocket Architecture` --semantically_similar_to--> `Five WebSocket Channels`  [INFERRED] [semantically similar]
  CLAUDE.md → api/asyncapi.yaml
- `WebSocket Streaming Protocol` --semantically_similar_to--> `GEX Faker WebSocket API`  [INFERRED] [semantically similar]
  WEBSOCKET.md → api/asyncapi.yaml
- `Wire-Compatible Orderflow Streaming` --semantically_similar_to--> `JSON-Protobuf-Zstd Encoding Pipeline`  [INFERRED] [semantically similar]
  docs/archived/PLAN_0003_30-11-2025_sun_websocket-orderflow-hub.md → WEBSOCKET.md
- `WebSocket Orderflow Hub Implementation Summary` --semantically_similar_to--> `GEX Faker WebSocket API`  [INFERRED] [semantically similar]
  docs/archived/SUMMARY_PLAN_0003_30-11-2025_sun_websocket-orderflow-hub.md → api/asyncapi.yaml

## Import Cycles
- None detected.

## Hyperedges (group relationships)
- **WebSocket Wire Protocol Flow** — websocket_azure_web_pubsub_protocol, websocket_encoding_pipeline, api_asyncapi_message_schemas, docs_archived_plan_0003_30_11_2025_sun_websocket_orderflow_hub_wire_compatible_streaming [INFERRED 0.95]
- **2026-08 endpoint compatibility findings** — compatibility_audit_2026_08_09_auth_parity, compatibility_audit_2026_08_09_gex_path_parity, compatibility_audit_2026_08_09_response_body_compat, compatibility_audit_2026_08_09_coverage_gaps [EXTRACTED 1.00]
- **Compatibility audit evidence chain** — compatibility_readme, compatibility_upstream_openapi, docs_gexbot_live_compatibility_audit [EXTRACTED 0.85]
- **GexBot Live Parity Remediation** — docs_gexbot_live_compatibility_audit_bearer_auth, docs_gexbot_live_compatibility_audit_negotiate_lifecycle, docs_gexbot_live_compatibility_audit_eod_report, docs_gexbot_live_compatibility_audit_futures_conversion [INFERRED 0.75]
- **Historical Data Download and Replay Lifecycle** — docs_archived_research_0001_29_11_2025_sat_quant_historical_analysis_historical_api_contract, docs_archived_plan_0001_29_11_2025_sat_gexbot_historical_downloader_atomic_download_pipeline, readme_historical_downloader, claude_data_loading [INFERRED 0.85]
- **Grafana reads from Prometheus and Loki datasources** — observability_grafana_service, observability_prometheus_service, observability_loki_service [EXTRACTED 0.95]
- **Log pipeline: Alloy ships container logs to Loki, Grafana visualizes** — observability_alloy_service, observability_loki_service, observability_grafana_service [INFERRED 0.85]
- **Alert delivery: metrics scraped, alerts evaluated, routed to ntfy** — observability_prometheus_service, grafana_alerting_rules_faker_health, observability_grafana_provisioning_alerting_contact_points_ntfy_receiver, ntfy_service [INFERRED 0.85]

## Communities (111 total, 39 thin omitted)

### Community 0 - "Generated API Types (1)"
Cohesion: 0.02
Nodes (92): AvailableDataResponse, AvailableDatesResponse, DataSummary, DownloadClassicGex404JSONResponse, DownloadLinksResponse, DownloadLinksSummary, DownloadOrderflow404JSONResponse, DownloadStateData404JSONResponse (+84 more)

### Community 1 - "WebSocket Hubs & Protobuf (1)"
Cohesion: 0.06
Nodes (49): Client, HistoryResponse, HTTPClient, Context, Duration, Logger, Writer, NewClient() (+41 more)

### Community 3 - "Configuration (1)"
Cohesion: 0.06
Nodes (42): convertCmd(), convertFile(), convertJSONToJSONL(), Command, downloadCmd(), Command, filterMarketDays(), generateTasks() (+34 more)

### Community 4 - "Daemon Entrypoint"
Cohesion: 0.07
Nodes (38): Calendar, getEnvBoolOrDefault(), getEnvIntOrDefault(), getEnvOrDefault(), LoadDaemonConfig(), T, TestDefaultSchedule(), convertFile() (+30 more)

### Community 5 - "Generated API Types (3)"
Cohesion: 0.09
Nodes (17): ChiServerOptions, MiddlewareFunc, ServerInterface, ServerInterfaceWrapper, strictHandler, StrictHTTPServerOptions, StrictServerInterface, HandlerFunc (+9 more)

### Community 6 - "EOD Archive Pipeline"
Cohesion: 0.12
Nodes (41): eodArchiveDates(), eodCmd(), eodDates(), eodMaterializeCmd(), eodPackCmd(), eodPruneCmd(), eodTickers(), eodVerifyCmd() (+33 more)

### Community 7 - "WebSocket Hubs & Protobuf (2)"
Cohesion: 0.06
Nodes (10): MiniContract, MiniContractPriors, OptionProfile, file_option_profile_proto_init(), file_option_profile_proto_rawDescGZIP(), Message, MessageState, SizeCache (+2 more)

### Community 8 - "WebSocket Hubs & Protobuf (3)"
Cohesion: 0.05
Nodes (4): MessageState, SizeCache, UnknownFields, Orderflow

### Community 9 - "HTTP Handlers & Routing (1)"
Cohesion: 0.13
Nodes (31): easterSunday(), generateExpiries(), Month, Time, isMarketDay(), isMarketHoliday(), lastWeekday(), nthWeekday() (+23 more)

### Community 10 - "HTTP Handlers & Routing (2)"
Cohesion: 0.13
Nodes (26): DownloadClassicGexRequestObject, DownloadClassicGexResponseObject, GetClassicGexChainResponseObject, GetClassicGexMajorsRequestObject, GetClassicGexMajorsResponseObject, GetClassicGexMaxChangeRequestObject, GetClassicGexMaxChangeResponseObject, GetDownloadLinksRequestObject (+18 more)

### Community 11 - "Generated API Types (4)"
Cohesion: 0.10
Nodes (6): DownloadClassicGexParamsAggregation, GetClassicGexMajorsParamsAggregation, GetClassicGexMaxChangeParamsAggregation, GetPackageCategoriesParamsPackage, Unimplemented, Request

### Community 12 - "Notifications"
Cohesion: 0.10
Nodes (20): BatchResult, FormatFailureMessage(), FormatSuccessMessage(), Duration, Config, Context, Duration, Logger (+12 more)

### Community 13 - "Data Loading & Cache (1)"
Cohesion: 0.11
Nodes (15): MemoryLoader, StreamLoader, File, DataKey(), extractTimestamp(), Context, GexData, Logger (+7 more)

### Community 14 - "Sync Broadcaster"
Cohesion: 0.14
Nodes (16): Flusher, Context, Duration, Logger, Request, ResponseWriter, RWMutex, maskAPIKey() (+8 more)

### Community 15 - "HTTP Handlers & Routing (3)"
Cohesion: 0.15
Nodes (24): GetFuturesConversionParams, GetFuturesConversionParamsFuture, GetFuturesConversionParamsModel, GetFuturesConversionParamsTicker, GetFuturesConversionRequestObject, GetFuturesConversionResponseObject, clTermination(), conversionParams() (+16 more)

### Community 16 - "Archived Plans"
Cohesion: 0.08
Nodes (26): Web PubSub Message Schemas, GEX Faker WebSocket API, Five WebSocket Channels, JSONL Data Loading and Playback, Five-Hub WebSocket Architecture, Default Downloader Configuration, GexBot Tickers, Packages, and Categories, Concurrent Atomic Download Pipeline (+18 more)

### Community 17 - "Download Manager"
Cohesion: 0.13
Nodes (14): Manager, mockClient, Task, TaskResult, Client, Context, Logger, NewManager() (+6 more)

### Community 18 - "HTTP Handlers & Routing (4)"
Cohesion: 0.14
Nodes (22): T, TestMaskQueryKeyRedactsSecret(), asyncapiHandler(), asyncapiUIHandler(), authMiddleware(), Context, Hub, Logger (+14 more)

### Community 19 - "Staging"
Cohesion: 0.12
Nodes (10): Client, Context, NewManager(), Context, T, Writer, TestStagingManager(), Downloader (+2 more)

### Community 20 - "Compatibility Audit (1)"
Cohesion: 0.23
Nodes (18): contains(), T, operationKey(), operationKeys(), readJSON(), readYAML(), schemaForOperation(), TestCompatibilityClassificationsMatchFakerSpec() (+10 more)

### Community 21 - "HTTP Handlers & Routing (5)"
Cohesion: 0.18
Nodes (12): buildDownloadPath(), categoryToPathParam(), f32ptr(), ResponseWriter, mapDataKeyToWSHubs(), parseDataKey(), classicDownloadResponse, downloadFileResponse (+4 more)

### Community 22 - "WebSocket Hubs & Protobuf (4)"
Cohesion: 0.12
Nodes (3): DownstreamMessage_DataMessage_, isMessageData_Data, MessageData

### Community 23 - "WebSocket Hubs & Protobuf (5)"
Cohesion: 0.16
Nodes (7): Client, Context, Logger, RWMutex, Hub, GroupMessage, GroupValidator

### Community 24 - "Observability Stack (1)"
Cohesion: 0.16
Nodes (17): Alert: Faker Daemon Download Failed, Alert: Faker Data Stale, Alert: Faker HTTP Error Rate High, Alert: Faker Reload Failed, Alert: Faker Target Down, Alert: Faker WebSocket Send Errors, Alert: Faker WebSocket Stalled, Grafana Faker Health Alert Group (+9 more)

### Community 26 - "WebSocket Hubs & Protobuf (7)"
Cohesion: 0.13
Nodes (3): SizeCache, DownstreamMessage_AckMessage_, DownstreamMessage_AckMessage_ErrorMessage

### Community 27 - "WebSocket Hubs & Protobuf (8)"
Cohesion: 0.13
Nodes (4): UnknownFields, DownstreamMessage, DownstreamMessage_PongMessage_, isDownstreamMessage_Message

### Community 28 - "WebSocket Hubs & Protobuf (9)"
Cohesion: 0.19
Nodes (8): extractGexTickerAndCategory(), Context, Duration, Hub, Logger, NewGexStreamer(), Encoder, GexStreamer

### Community 29 - "Compatibility Audit (2)"
Cohesion: 0.15
Nodes (15): GexBot compatibility evidence README, Upstream GexBot OpenAPI contract, GET /{package}/categories, GET /{ticker}/classic/{category}, GET /options/{ticker}/expiries, GET /futures/conversion, GET /hist/{ticker}/{package}/{category}/{date}, GET /hist/eod/{ticker} (+7 more)

### Community 30 - "Data Loading & Cache (2)"
Cohesion: 0.18
Nodes (6): DataLoader, ReloadableLoader, Context, GexData, RWMutex, NewReloadableLoader()

### Community 31 - "HTTP Handlers & Routing (6)"
Cohesion: 0.22
Nodes (9): Context, Logger, RWMutex, Time, isValidDateFormat(), NewReloadManager(), Mutex, ReloadManager (+1 more)

### Community 32 - "docs"
Cohesion: 0.16
Nodes (14): gex-tools Service (tools profile), GexBot Live Compatibility Audit, Bearer Auth and Required Headers, go test ./compatibility check, Drop-in Replacement Goal, Endpoint Compatibility Matrix, Live EOD Report Endpoint, Futures Conversion Endpoint (+6 more)

### Community 33 - "OBSERVABILITY"
Cohesion: 0.30
Nodes (14): observability Compose Profile, Grafana Alloy, Caddy Observability Gateway, Gateway Basic Auth, Faker Observability Dashboard Provider, Grafana Loki Datasource, Grafana Prometheus Datasource, Grafana (+6 more)

### Community 34 - "WebSocket Hubs & Protobuf (10)"
Cohesion: 0.18
Nodes (4): MaxPriors, MaxPriorsTuple, SizeCache, UnknownFields

### Community 35 - "WebSocket Hubs & Protobuf (11)"
Cohesion: 0.18
Nodes (10): Conn, Hub, Logger, Request, ResponseWriter, RWMutex, Hub, buildConnectedMessage() (+2 more)

### Community 36 - "WebSocket Hubs & Protobuf (12)"
Cohesion: 0.20
Nodes (9): buildAckMessage(), buildAckMessageJSON(), buildPongMessage(), buildPongMessageJSON(), parseUpstreamMessage(), parseUpstreamMessageJSON(), joinGroupRequest, leaveGroupRequest (+1 more)

### Community 37 - "WebSocket Hubs & Protobuf (13)"
Cohesion: 0.18
Nodes (4): MessageState, isUpstreamMessage_Message, UpstreamMessage, UpstreamMessage_PingMessage_

### Community 38 - "Ticker Sync CLI"
Cohesion: 0.23
Nodes (9): Bool, main(), run(), fatal(), main(), normalize(), T, TestNormalizeSortsAndIsDeterministic() (+1 more)

### Community 39 - "Observability Metrics (1)"
Cohesion: 0.21
Nodes (8): Context, Logger, Server, NewDiagnostics(), statusHandler(), T, TestStatusHandler(), Diagnostics

### Community 40 - "WebSocket Hubs & Protobuf (14)"
Cohesion: 0.20
Nodes (6): Any, file_webpubsub_messages_proto_init(), init(), MessageData_BinaryData, MessageData_ProtobufData, MessageData_TextData

### Community 41 - "Compatibility Audit (3)"
Cohesion: 0.18
Nodes (11): getClassicGexChain endpoint, getOrderflowLatest endpoint, getStateProfile endpoint, getTickers endpoint, Endpoint Compatibility Audit (2026-08-09), Auth mechanism parity (header vs query key), GEX path segment parity (gex_ prefix), Live probe as compatibility oracle (+3 more)

### Community 42 - "Data Loading & Cache (3)"
Cohesion: 0.25
Nodes (4): CacheMode, IndexCache, RWMutex, NewIndexCache()

### Community 43 - "WebSocket Hubs & Protobuf (15)"
Cohesion: 0.33
Nodes (8): WSCacheKey(), extractClassicTickerAndCategory(), Context, Duration, Hub, Logger, NewClassicStreamer(), ClassicStreamer

### Community 44 - "WebSocket Hubs & Protobuf (16)"
Cohesion: 0.36
Nodes (8): extractTicker(), Context, Duration, Hub, Logger, NewStreamer(), ReloadChecker, Streamer

### Community 45 - "WebSocket Hubs & Protobuf (17)"
Cohesion: 0.38
Nodes (7): extractGreekOneTickerAndCategory(), Context, Duration, Hub, Logger, NewGreekOneStreamer(), GreekOneStreamer

### Community 46 - "WebSocket Hubs & Protobuf (18)"
Cohesion: 0.38
Nodes (7): extractGreekTickerAndCategory(), Context, Duration, Hub, Logger, NewGreekStreamer(), GreekStreamer

### Community 53 - "Generated API Types (5)"
Cohesion: 0.22
Nodes (5): DownloadClassicGex200ApplicationxNdjsonResponse, DownloadOrderflow200ApplicationxNdjsonResponse, DownloadStateData200ApplicationxNdjsonResponse, GetHistEod200ApplicationzipResponse, Reader

### Community 56 - "Configuration (2)"
Cohesion: 0.39
Nodes (7): ServerConfig, detectLatestDate(), getEnvOrDefault(), Duration, LoadServerConfig(), newestDateDir(), Regexp

### Community 57 - "Observability Stack (2)"
Cohesion: 0.29
Nodes (8): gex-daemon Service, ntfy Notification Service, Grafana Notification Routing Policy, ntfy Contact Point (Grafana webhook), Scrape Job: faker-api (gex-faker-api:9090), Scrape Job: faker-daemon (gex-daemon:9091), Prometheus Scrape Config, Daemon Service (scheduled EOD downloads)

### Community 58 - "Generated API Types (6)"
Cohesion: 0.25
Nodes (6): GetCurrentDateRequestObject, GetCurrentDateResponseObject, ReloadDateRequestObject, ReloadDateResponseObject, Time, ReloadDateJSONRequestBody

### Community 60 - "WebSocket Hubs & Protobuf (28)"
Cohesion: 0.36
Nodes (4): buildDataMessage(), buildDataMessageJSON(), T, TestDataMessageFromParity()

### Community 61 - "API Layer (1)"
Cohesion: 0.29
Nodes (7): getFuturesConversion endpoint, getHistEod endpoint, getHistSnapshot endpoint, getOptionsExpiries endpoint, getPackageCategories endpoint, getTickersQuant endpoint, Coverage gaps (8 upstream endpoints not served)

### Community 64 - "README"
Cohesion: 0.53
Nodes (6): gex-faker-api Service, Faker Control-Plane Extensions, GEX Faker API, Hot Reload Data Dates, Sync Broadcast System (SSE), WebSocket Streaming Hubs (5 hubs)

### Community 65 - "Generated API Types (7)"
Cohesion: 0.40
Nodes (5): GetHealthRequestObject, GetHealthResponseObject, HealthResponse, HealthResponseCacheMode, HealthResponseDataMode

### Community 66 - "Generated API Types (8)"
Cohesion: 0.47
Nodes (3): GetHistSnapshotParams, GetHistSnapshotParamsPackage, GetHistSnapshotRequestObject

### Community 71 - "Data Loading & Cache (4)"
Cohesion: 0.50
Nodes (4): GexData, GreekData, OrderflowData, RawMessage

### Community 72 - "Generated API Types (9)"
Cohesion: 0.40
Nodes (4): GetOptionsExpiriesRequestObject, GetOptionsExpiriesResponseObject, Context, Server

### Community 73 - "WebSocket Hubs & Protobuf (35)"
Cohesion: 0.50
Nodes (3): file_orderflow_proto_init(), file_orderflow_proto_rawDescGZIP(), init()

### Community 74 - "Generated API Types (10)"
Cohesion: 0.50
Nodes (3): GetAvailableDataParams, GetAvailableDataRequestObject, GetAvailableDataResponseObject

### Community 75 - "Generated API Types (11)"
Cohesion: 0.50
Nodes (3): GetPackageCategories200JSONResponse, GetPackageCategoriesRequestObject, GetPackageCategoriesResponseObject

### Community 77 - "Generated API Types (12)"
Cohesion: 0.67
Nodes (3): CurrentDateResponse, ReloadDateResponse, Time

### Community 88 - "Generated API Types (23)"
Cohesion: 0.67
Nodes (3): GetSwagger(), T, PathToRawSpec()

## Knowledge Gaps
- **84 isolated node(s):** `matrixEntry`, `liveProbeDoc`, `github.com/dgnsrekt/gexbot-downloader`, `HistoryResponse`, `AvailableDatesResponse` (+79 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **39 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `run()` connect `Ticker Sync CLI` to `WebSocket Hubs & Protobuf (1)`, `EOD Archive Pipeline`, `Observability Metrics (1)`, `Data Loading & Cache (3)`, `WebSocket Hubs & Protobuf (15)`, `WebSocket Hubs & Protobuf (16)`, `Data Loading & Cache (1)`, `Sync Broadcaster`, `WebSocket Hubs & Protobuf (17)`, `WebSocket Hubs & Protobuf (18)`, `HTTP Handlers & Routing (4)`, `Configuration (2)`, `WebSocket Hubs & Protobuf (9)`, `Data Loading & Cache (2)`, `HTTP Handlers & Routing (6)`?**
  _High betweenness centrality (0.078) - this node is a cross-community bridge._
- **Why does `run()` connect `Daemon Entrypoint` to `Configuration (1)`, `Notifications`, `Ticker Sync CLI`, `Observability Metrics (1)`?**
  _High betweenness centrality (0.070) - this node is a cross-community bridge._
- **Why does `New()` connect `Notifications` to `WebSocket Hubs & Protobuf (1)`, `WebSocket Hubs & Protobuf (11)`, `Configuration (1)`, `Daemon Entrypoint`, `EOD Archive Pipeline`?**
  _High betweenness centrality (0.062) - this node is a cross-community bridge._
- **Are the 67 inferred relationships involving `NewEncoder()` (e.g. with `.VisitDownloadClassicGexResponse()` and `.VisitDownloadOrderflowResponse()`) actually correct?**
  _`NewEncoder()` has 67 INFERRED edges - model-reasoned connections that need verification._
- **Are the 37 inferred relationships involving `HandlerFunc` (e.g. with `.DownloadClassicGex()` and `.DownloadOrderflow()`) actually correct?**
  _`HandlerFunc` has 37 INFERRED edges - model-reasoned connections that need verification._
- **What connects `matrixEntry`, `liveProbeDoc`, `github.com/dgnsrekt/gexbot-downloader` to the rest of the system?**
  _84 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `Generated API Types (1)` be split into smaller, more focused modules?**
  _Cohesion score 0.024544179523141654 - nodes in this community are weakly interconnected._