package indicators

import "math"

type Bar struct {
	Date   string
	Open   float64
	High   float64
	Low    float64
	Close  float64
	Volume float64
}

func SMA(bars []Bar, period int) float64 {
	if len(bars) < period {
		return math.NaN()
	}
	sum := 0.0
	for _, b := range bars[len(bars)-period:] {
		sum += b.Close
	}
	return sum / float64(period)
}

// AvgVolume - average volume over the last `period` bars.
func AvgVolume(bars []Bar, period int) float64 {
	if len(bars) < period {
		return math.NaN()
	}
	sum := 0.0
	for _, b := range bars[len(bars)-period:] {
		sum += b.Volume
	}
	return sum / float64(period)
}

// DonchianHigh - highest High over the last `period` bars, EXCLUDING the
// most recent bar (so "today's close breaks the prior N-day high" is a
// clean comparison, not comparing today against itself).
func DonchianHigh(bars []Bar, period int) float64 {
	if len(bars) < period+1 {
		return math.NaN()
	}
	window := bars[len(bars)-period-1 : len(bars)-1]
	high := window[0].High
	for _, b := range window {
		if b.High > high {
			high = b.High
		}
	}
	return high
}

// ATR - Average True Range, Wilder's smoothing, over `period`.
func ATR(bars []Bar, period int) float64 {
	if len(bars) < period+1 {
		return math.NaN()
	}
	trs := make([]float64, 0, len(bars)-1)
	for i := 1; i < len(bars); i++ {
		high, low, prevClose := bars[i].High, bars[i].Low, bars[i-1].Close
		tr := math.Max(high-low, math.Max(math.Abs(high-prevClose), math.Abs(low-prevClose)))
		trs = append(trs, tr)
	}
	// seed with simple average of first `period` TRs
	sum := 0.0
	for _, tr := range trs[:period] {
		sum += tr
	}
	atr := sum / float64(period)
	// Wilder smoothing for the rest
	for _, tr := range trs[period:] {
		atr = (atr*float64(period-1) + tr) / float64(period)
	}
	return atr
}

// ATRPercent - ATR as a percentage of current close, for cross-sectional
// comparison (used to skip the most gap-prone names).
func ATRPercent(bars []Bar, period int) float64 {
	atr := ATR(bars, period)
	if math.IsNaN(atr) || len(bars) == 0 {
		return math.NaN()
	}
	return atr / bars[len(bars)-1].Close
}

// RateOfChange - simple % return over `period` bars, used as the raw input
// to relative strength ranking across the universe.
func RateOfChange(bars []Bar, period int) float64 {
	if len(bars) < period+1 {
		return math.NaN()
	}
	past := bars[len(bars)-period-1].Close
	current := bars[len(bars)-1].Close
	if past == 0 {
		return math.NaN()
	}
	return (current - past) / past * 100
}
