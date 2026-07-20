package positions

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/Tanish431/us-momentum-scanner/internal/config"
	"github.com/Tanish431/us-momentum-scanner/internal/indicators"
)

type Position struct {
	Ticker        string  `json:"ticker"`
	EntryDate     string  `json:"entry_date"`
	EntryPrice    float64 `json:"entry_price"`
	Shares        int     `json:"shares"`
	InitialStop   float64 `json:"initial_stop"`
	CurrentStop   float64 `json:"current_stop"`
	StopOrderID   string  `json:"stop_order_id"`
	WeeksOpen     int     `json:"weeks_open"`
	LastCheckDate string  `json:"last_check_date"`
}

type Book struct {
	path      string
	Positions map[string]*Position `json:"positions"`
}

func LoadBook(dataDir string) (*Book, error) {
	p := filepath.Join(dataDir, "open_positions.json")
	b := &Book{path: p, Positions: map[string]*Position{}}
	data, err := os.ReadFile(p)
	if os.IsNotExist(err) {
		return b, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, &b.Positions); err != nil {
		return nil, err
	}
	return b, nil
}

func (b *Book) Save() error {
	if err := os.MkdirAll(filepath.Dir(b.path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(b.Positions, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(b.path, data, 0644)
}

func (b *Book) Open(ticker string, entryPrice, initialStop float64, shares int, date string, stopOrderID string) {
	b.Positions[ticker] = &Position{
		Ticker:        ticker,
		EntryDate:     date,
		EntryPrice:    entryPrice,
		Shares:        shares,
		InitialStop:   initialStop,
		CurrentStop:   initialStop,
		StopOrderID:   stopOrderID,
		WeeksOpen:     0,
		LastCheckDate: date,
	}
}

func (b *Book) Close(ticker string) {
	delete(b.Positions, ticker)
}

func (b *Book) IsOpen(ticker string) bool {
	_, ok := b.Positions[ticker]
	return ok
}

func (b *Book) Count() int {
	return len(b.Positions)
}

type ExitReason string

const (
	NoExit   ExitReason = ""
	StopHit  ExitReason = "stop_hit"
	TimeStop ExitReason = "time_stop"
)

func Update(pos *Position, bars []indicators.Bar, cfg config.Strategy, today time.Time) ExitReason {
	if len(bars) == 0 {
		return NoExit
	}
	last := bars[len(bars)-1]

	// stop-out check first
	if last.Low <= pos.CurrentStop {
		return StopHit
	}

	// ratchet trail upward only, never down
	atr := indicators.ATR(bars, cfg.ATRPeriod)
	if !isNaN(atr) {
		candidate := last.Close - atr*cfg.TrailATRMult
		if candidate > pos.CurrentStop {
			pos.CurrentStop = candidate
		}
	}
	if today.Weekday() == time.Friday && pos.LastCheckDate != today.Format("2006-01-02") {
		pos.WeeksOpen++
		pos.LastCheckDate = today.Format("2006-01-02")
		if pos.WeeksOpen >= cfg.TimeStopWeeks && last.Close <= pos.EntryPrice {
			return TimeStop
		}
	}

	return NoExit
}

func isNaN(f float64) bool { return f != f }
