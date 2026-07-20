package cache

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"

	"github.com/Tanish431/us-momentum-scanner/internal/indicators"
)

type Store struct {
	Dir string
}

func NewStore(dir string) *Store {
	return &Store{Dir: dir}
}

func (s *Store) path(ticker string) string {
	return filepath.Join(s.Dir, ticker+".json")
}

// Load reads cached bars for a ticker. Returns an empty slice (not an
// error) if no cache file exists yet — first run for that ticker.
func (s *Store) Load(ticker string) ([]indicators.Bar, error) {
	data, err := os.ReadFile(s.path(ticker))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var bars []indicators.Bar
	if err := json.Unmarshal(data, &bars); err != nil {
		return nil, err
	}
	return bars, nil
}

// Save writes the full bar history for a ticker, overwriting the file.
func (s *Store) Save(ticker string, bars []indicators.Bar) error {
	if err := os.MkdirAll(s.Dir, 0755); err != nil {
		return err
	}
	data, err := json.Marshal(bars)
	if err != nil {
		return err
	}
	return os.WriteFile(s.path(ticker), data, 0644)
}

// Merge combines cached bars with freshly fetched bars, de-duplicating by
// date (fresh data wins — handles the case where the last cached day was
// an intraday partial bar that's now final) and keeping the result sorted.
func Merge(cached, fresh []indicators.Bar) []indicators.Bar {
	byDate := make(map[string]indicators.Bar, len(cached)+len(fresh))
	for _, b := range cached {
		byDate[b.Date] = b
	}
	for _, b := range fresh {
		byDate[b.Date] = b // fresh overwrites cached on same date
	}
	merged := make([]indicators.Bar, 0, len(byDate))
	for _, b := range byDate {
		merged = append(merged, b)
	}
	sort.Slice(merged, func(i, j int) bool { return merged[i].Date < merged[j].Date })
	return merged
}

// LastDate returns the most recent date in a bar slice, or "" if empty.
func LastDate(bars []indicators.Bar) string {
	if len(bars) == 0 {
		return ""
	}
	last := bars[0].Date
	for _, b := range bars {
		if b.Date > last {
			last = b.Date
		}
	}
	return last
}
