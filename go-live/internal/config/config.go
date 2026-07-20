package config

import (
	"os"
	"strconv"
)

type Strategy struct {
	BreakoutLookbackDays int
	VolumeAvgDays        int
	VolumeMultiplier     float64
	RSLookbackDays       int
	RSPercentileMin      float64
	TrendFastSMA         int
	TrendSlowSMA         int
	ATRPeriod            int
	ATRPctSkipDecile     float64
	InitialStopATRMult   float64
	TrailATRMult         float64
	TrailUseSMA          bool
	TrailSMAPeriod       int
	TimeStopWeeks        int
	RiskPerTradePct      float64
	MaxConcurrentPos     int
}

func Locked() Strategy {
	return Strategy{
		BreakoutLookbackDays: 20,
		VolumeAvgDays:        50,
		VolumeMultiplier:     1.5,
		RSLookbackDays:       63,
		RSPercentileMin:      80, // TODO: confirm via backtest
		TrendFastSMA:         50,
		TrendSlowSMA:         200,
		ATRPeriod:            14,
		ATRPctSkipDecile:     0.90,

		InitialStopATRMult: 2.0,
		TrailATRMult:       3.0, // TODO: confirm via backtest
		TrailUseSMA:        false,
		TrailSMAPeriod:     20,
		TimeStopWeeks:      8, // TODO: confirm via backtest

		RiskPerTradePct:  0.75,
		MaxConcurrentPos: 5,
	}
}

type Runtime struct {
	AlpacaKeyID     string
	AlpacaSecretKey string
	AlpacaDataURL   string
	AlpacaTradeURL  string
	DataDir         string
	AccountEquity   float64
}

func LoadRuntime() Runtime {
	equity, _ := strconv.ParseFloat(getenv("ACCOUNT_EQUITY", "10000"), 64)
	return Runtime{
		AlpacaKeyID:     os.Getenv("ALPACA_KEY_ID"),
		AlpacaSecretKey: os.Getenv("ALPACA_SECRET_KEY"),
		AlpacaDataURL:   getenv("ALPACA_DATA_URL", "https://data.alpaca.markets/v2"),
		AlpacaTradeURL:  getenv("ALPACA_TRADE_URL", "https://paper-api.alpaca.markets"),
		DataDir:         getenv("DATA_DIR", "./data"),
		AccountEquity:   equity,
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
