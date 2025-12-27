package sync

// SyncBatch represents a batch of position updates broadcast to subscribers.
type SyncBatch struct {
	BroadcasterID string         `json:"broadcaster_id"`
	DataDate      string         `json:"data_date"`
	CacheMode     string         `json:"cache_mode"`
	Timestamp     int64          `json:"timestamp"`
	Sequence      uint64         `json:"sequence"`
	Positions     []SyncPosition `json:"positions"`
}

// SyncPosition represents a single cache position update.
type SyncPosition struct {
	CacheKey      string `json:"cache_key"`
	Index         int    `json:"index"`
	DataLength    int    `json:"data_length"`
	DataTimestamp int64  `json:"data_timestamp"`
	Exhausted     bool   `json:"exhausted"`
}

// SyncSnapshot represents complete state for a new subscriber connection.
// It's structurally identical to SyncBatch but semantically represents initial state.
type SyncSnapshot struct {
	BroadcasterID string         `json:"broadcaster_id"`
	DataDate      string         `json:"data_date"`
	CacheMode     string         `json:"cache_mode"`
	Timestamp     int64          `json:"timestamp"`
	Sequence      uint64         `json:"sequence"`
	Positions     []SyncPosition `json:"positions"`
}
