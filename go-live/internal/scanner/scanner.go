package scanner

import (
	"sort"

	"github.com/Tanish431/us-momentum-scanner/internal/config"
	"github.com/Tanish431/us-momentum-scanner/internal/indicators"
)

type Symbol struct {
	Ticker string
	Bars   []indicators.Bar
}

type Signal struct {
	Ticker       string
	EntryPrice   float64
	InitialStop  float64
	ATR          float64
	RSPercentile float64
}

type EvalResult struct {
	Ticker      string
	FailedStage string // "breakout", "volume", "rs", "trend", "atr_skip", "passed"
}

// RankRelativeStrength computes RS percentile (0-100) for every symbol in
// the universe based on RateOfChange, then returns a ticker->percentile map.
// Must be run across the full universe together, not per-symbol.
func RankRelativeStrength(universe []Symbol, lookback int) map[string]float64 {
	type roc struct {
		ticker string
		value  float64
	}
	rocs := make([]roc, 0, len(universe))
	for _, s := range universe {
		v := indicators.RateOfChange(s.Bars, lookback)
		if !isNaN(v) {
			rocs = append(rocs, roc{s.Ticker, v})
		}
	}
	sort.Slice(rocs, func(i, j int) bool { return rocs[i].value < rocs[j].value })

	result := make(map[string]float64, len(rocs))
	n := len(rocs)
	for i, r := range rocs {
		result[r.ticker] = float64(i) / float64(n-1) * 100
	}
	return result
}

// Evaluate checks a single symbol against every entry filter. Returns
// (signal, true) if all conditions pass, else (zero, false).
func Evaluate(s Symbol, rsPercentile float64, cfg config.Strategy) (Signal, bool) {
	bars := s.Bars
	if len(bars) < cfg.TrendSlowSMA+1 {
		return Signal{}, false
	}
	last := bars[len(bars)-1]

	// 1. Breakout: today's close beats the prior N-day high
	donchian := indicators.DonchianHigh(bars, cfg.BreakoutLookbackDays)
	if isNaN(donchian) || last.Close <= donchian {
		return Signal{}, false
	}

	// 2. Volume confirmation
	avgVol := indicators.AvgVolume(bars[:len(bars)-1], cfg.VolumeAvgDays)
	if isNaN(avgVol) || last.Volume < avgVol*cfg.VolumeMultiplier {
		return Signal{}, false
	}

	// 3. Relative strength - top bucket only
	if rsPercentile < cfg.RSPercentileMin {
		return Signal{}, false
	}

	// 4. Trend filter: close > SMA50 > SMA200, both established as uptrend
	smaFast := indicators.SMA(bars, cfg.TrendFastSMA)
	smaSlow := indicators.SMA(bars, cfg.TrendSlowSMA)
	if isNaN(smaFast) || isNaN(smaSlow) {
		return Signal{}, false
	}
	if !(last.Close > smaFast && smaFast > smaSlow) {
		return Signal{}, false
	}

	// 5. Volatility skip - avoid extreme ATR% names (set threshold from
	// universe-wide ATR% distribution upstream; placeholder direct check
	// left to caller since it requires cross-sectional context)
	atr := indicators.ATR(bars, cfg.ATRPeriod)
	if isNaN(atr) {
		return Signal{}, false
	}

	stop := last.Close - atr*cfg.InitialStopATRMult

	return Signal{
		Ticker:       s.Ticker,
		EntryPrice:   last.Close,
		InitialStop:  stop,
		ATR:          atr,
		RSPercentile: rsPercentile,
	}, true
}

func EvaluateVerbose(s Symbol, rsPercentile float64, cfg config.Strategy) (Signal, EvalResult) {
	bars := s.Bars
	if len(bars) < cfg.TrendSlowSMA+1 {
		return Signal{}, EvalResult{s.Ticker, "insufficient_history"}
	}
	last := bars[len(bars)-1]

	donchian := indicators.DonchianHigh(bars, cfg.BreakoutLookbackDays)
	if isNaN(donchian) || last.Close <= donchian {
		return Signal{}, EvalResult{s.Ticker, "breakout"}
	}

	avgVol := indicators.AvgVolume(bars[:len(bars)-1], cfg.VolumeAvgDays)
	if isNaN(avgVol) || last.Volume < avgVol*cfg.VolumeMultiplier {
		return Signal{}, EvalResult{s.Ticker, "volume"}
	}

	if rsPercentile < cfg.RSPercentileMin {
		return Signal{}, EvalResult{s.Ticker, "rs"}
	}

	smaFast := indicators.SMA(bars, cfg.TrendFastSMA)
	smaSlow := indicators.SMA(bars, cfg.TrendSlowSMA)
	if isNaN(smaFast) || isNaN(smaSlow) || !(last.Close > smaFast && smaFast > smaSlow) {
		return Signal{}, EvalResult{s.Ticker, "trend"}
	}

	atr := indicators.ATR(bars, cfg.ATRPeriod)
	if isNaN(atr) {
		return Signal{}, EvalResult{s.Ticker, "atr_missing"}
	}

	stop := last.Close - atr*cfg.InitialStopATRMult
	return Signal{
		Ticker: s.Ticker, EntryPrice: last.Close, InitialStop: stop,
		ATR: atr, RSPercentile: rsPercentile,
	}, EvalResult{s.Ticker, "passed"}
}

func isNaN(f float64) bool { return f != f }
