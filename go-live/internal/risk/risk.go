package risk

// PositionSize computes share count for a given account equity, risk
// percentage per trade, entry price, and stop price. Unchanged logic from
// the NSE system - this part is account math, not market-specific.
func PositionSize(equity, riskPct, entry, stop float64) int {
	if entry <= stop || equity <= 0 {
		return 0
	}
	riskAmount := equity * (riskPct / 100)
	perShareRisk := entry - stop
	shares := int(riskAmount / perShareRisk)
	if shares < 0 {
		return 0
	}
	return shares
}

// TrailStop computes the new trailing stop given current price, ATR, and
// the configured trail multiplier. Only moves up, never down (standard
// trailing-stop ratchet).
func TrailStop(currentStop, price, atr, trailMult float64) float64 {
	candidate := price - atr*trailMult
	if candidate > currentStop {
		return candidate
	}
	return currentStop
}
