package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/Tanish431/us-momentum-scanner/internal/scanner"
)

type SignalRecord struct {
	Timestamp    string  `json:"timestamp"`
	Ticker       string  `json:"ticker"`
	EntryPrice   float64 `json:"entry_price"`
	InitialStop  float64 `json:"initial_stop"`
	ATR          float64 `json:"atr"`
	RSPercentile float64 `json:"rs_percentile"`
	Shares       int     `json:"shares"`
}

type Logger struct {
	DataDir string
}

func NewLogger(dataDir string) *Logger {
	return &Logger{DataDir: dataDir}
}

// AppendSignals writes today's signals to a daily JSONL file, same pattern
// as the NSE scanner's live log — one file per day, append-only.
func (l *Logger) AppendSignals(signals []scanner.Signal, sharesByTicker map[string]int) error {
	if err := os.MkdirAll(l.DataDir, 0755); err != nil {
		return err
	}

	today := time.Now().Format("2006-01-02")
	path := filepath.Join(l.DataDir, today+"_signals.jsonl")

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	now := time.Now().Format(time.RFC3339)
	enc := json.NewEncoder(f)
	for _, s := range signals {
		rec := SignalRecord{
			Timestamp:    now,
			Ticker:       s.Ticker,
			EntryPrice:   s.EntryPrice,
			InitialStop:  s.InitialStop,
			ATR:          s.ATR,
			RSPercentile: s.RSPercentile,
			Shares:       sharesByTicker[s.Ticker],
		}
		if err := enc.Encode(rec); err != nil {
			return err
		}
	}
	return nil
}
