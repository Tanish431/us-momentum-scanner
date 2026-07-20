package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/Tanish431/us-momentum-scanner/internal/broker"
	"github.com/Tanish431/us-momentum-scanner/internal/cache"
	"github.com/Tanish431/us-momentum-scanner/internal/config"
	"github.com/Tanish431/us-momentum-scanner/internal/data"
	"github.com/Tanish431/us-momentum-scanner/internal/indicators"
	"github.com/Tanish431/us-momentum-scanner/internal/positions"
	"github.com/Tanish431/us-momentum-scanner/internal/risk"
	"github.com/Tanish431/us-momentum-scanner/internal/scanner"
	"github.com/Tanish431/us-momentum-scanner/internal/storage"
)

func loadTickers(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var raw []map[string]string
	if err := json.NewDecoder(f).Decode(&raw); err != nil {
		return nil, fmt.Errorf("expected JSON array of {\"ticker\": \"AAPL\"}: %w", err)
	}
	tickers := make([]string, 0, len(raw))
	for _, r := range raw {
		tickers = append(tickers, r["ticker"])
	}
	return tickers, nil
}

func main() {
	cfg := config.Locked()
	rt := config.LoadRuntime()

	if rt.AlpacaKeyID == "" || rt.AlpacaSecretKey == "" {
		fmt.Println("ALPACA_KEY_ID / ALPACA_SECRET_KEY not set")
		os.Exit(1)
	}

	tickers, err := loadTickers(rt.DataDir + "/universe/sp500.json")
	if err != nil {
		fmt.Printf("failed to load universe: %v\n", err)
		os.Exit(1)
	}

	client := data.NewClient(rt.AlpacaKeyID, rt.AlpacaSecretKey, rt.AlpacaDataURL)
	store := cache.NewStore(rt.DataDir + "/cache")
	brokerClient := broker.NewClient(rt.AlpacaKeyID, rt.AlpacaSecretKey, rt.AlpacaTradeURL)

	now := time.Now()
	today := now.Format("2006-01-02")
	lookbackFloor := now.AddDate(-1, 0, 0).Format("2006-01-02") // covers SMA200 + RS lookback

	// ---------- 1. Fetch bars, cache-aware ----------
	fmt.Printf("Fetching bars for %d tickers (cache-aware)...\n", len(tickers))

	var universe []scanner.Symbol
	barsByTicker := make(map[string][]indicators.Bar, len(tickers))

	for i, ticker := range tickers {
		cached, err := store.Load(ticker)
		if err != nil {
			fmt.Printf("[cache error] %s: %v\n", ticker, err)
			continue
		}

		fetchStart := lookbackFloor
		if last := cache.LastDate(cached); last != "" && last > lookbackFloor {
			fetchStart = last
		}

		fresh, err := client.GetDailyBars(ticker, fetchStart, today)
		if err != nil {
			fmt.Printf("[skip] %s: %v\n", ticker, err)
			if len(cached) > 0 {
				universe = append(universe, scanner.Symbol{Ticker: ticker, Bars: cached})
				barsByTicker[ticker] = cached
			}
			continue
		}

		merged := cache.Merge(cached, fresh)
		if err := store.Save(ticker, merged); err != nil {
			fmt.Printf("[cache save error] %s: %v\n", ticker, err)
		}

		universe = append(universe, scanner.Symbol{Ticker: ticker, Bars: merged})
		barsByTicker[ticker] = merged

		time.Sleep(350 * time.Millisecond)
		if (i+1)%50 == 0 {
			fmt.Printf("-- processed %d/%d --\n", i+1, len(tickers))
		}
	}

	// ---------- 2. Load and update open positions ----------
	book, err := positions.LoadBook(rt.DataDir)
	if err != nil {
		fmt.Printf("failed to load position book: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\nChecking open positions...")
	for ticker, pos := range book.Positions {
		status, err := brokerClient.GetOrderStatus(pos.StopOrderID)
		if err != nil {
			fmt.Printf("  [warn] %s: couldn't check stop order: %v\n", ticker, err)
			continue
		}
		if status == "filled" {
			fmt.Printf("  STOPPED OUT %s at ~%.2f\n", ticker, pos.CurrentStop)
			book.Close(ticker)
			continue
		}

		bars, ok := barsByTicker[ticker]
		if !ok {
			fmt.Printf("  [warn] no fresh bars for %s, skipping trail update\n", ticker)
			continue
		}
		last := bars[len(bars)-1]
		atr := indicators.ATR(bars, cfg.ATRPeriod)

		// time-stop check
		isFriday := now.Weekday() == time.Friday && pos.LastCheckDate != today
		if isFriday {
			pos.WeeksOpen++
			pos.LastCheckDate = today
		}
		if pos.WeeksOpen >= cfg.TimeStopWeeks && last.Close <= pos.EntryPrice {
			fmt.Printf("  TIME STOP %s: closing at market\n", ticker)
			brokerClient.CancelOrder(pos.StopOrderID)
			if _, err := brokerClient.PlaceMarketOrder(ticker, pos.Shares, "sell"); err != nil {
				fmt.Printf("  [error] failed to close %s: %v\n", ticker, err)
			}
			book.Close(ticker)
			continue
		}

		// trail ratchet — only replace the order if the new stop is actually higher
		if atr == atr { // not NaN
			newStop := last.Close - atr*cfg.TrailATRMult
			if newStop > pos.CurrentStop {
				if err := brokerClient.CancelOrder(pos.StopOrderID); err != nil {
					fmt.Printf("  [warn] %s: cancel old stop failed: %v\n", ticker, err)
				}
				newOrder, err := brokerClient.PlaceStopSell(ticker, pos.Shares, newStop)
				if err != nil {
					fmt.Printf("  [error] %s: failed to place new stop: %v\n", ticker, err)
				} else {
					pos.CurrentStop = newStop
					pos.StopOrderID = newOrder.ID
					fmt.Printf("  trailed %s stop to %.2f\n", ticker, newStop)
				}
			} else {
				fmt.Printf("  hold %s: stop=%.2f weeks_open=%d\n", ticker, pos.CurrentStop, pos.WeeksOpen)
			}
		}
	}

	availableSlots := cfg.MaxConcurrentPos - book.Count()
	fmt.Printf("Open positions: %d / %d (slots available: %d)\n", book.Count(), cfg.MaxConcurrentPos, availableSlots)

	// ---------- 3. Rank relative strength across the universe ----------
	fmt.Println("\nRanking relative strength...")
	rsRanks := scanner.RankRelativeStrength(universe, cfg.RSLookbackDays)

	// ---------- 4. Scan for new entries in available slots ----------
	fmt.Println("Evaluating entries...")

	var signals []scanner.Signal
	sharesByTicker := make(map[string]int)
	stageCounts := map[string]int{}

	if availableSlots <= 0 {
		fmt.Println("No available slots, skipping new entries.")
	} else {
		for _, sym := range universe {
			if book.IsOpen(sym.Ticker) {
				continue
			}

			rsPct, ok := rsRanks[sym.Ticker]
			if !ok {
				continue
			}

			sig, result := scanner.EvaluateVerbose(sym, rsPct, cfg)
			stageCounts[result.FailedStage]++
			if result.FailedStage != "passed" {
				continue
			}

			shares := risk.PositionSize(rt.AccountEquity, cfg.RiskPerTradePct, sig.EntryPrice, sig.InitialStop)
			if shares <= 0 {
				continue
			}

			buyOrder, err := brokerClient.PlaceMarketOrder(sig.Ticker, shares, "buy")
			if err != nil {
				fmt.Printf("  [error] failed to buy %s: %v\n", sig.Ticker, err)
				continue
			}
			stopOrder, err := brokerClient.PlaceStopSell(sig.Ticker, shares, sig.InitialStop)
			if err != nil {
				fmt.Printf("  [error] bought %s but failed to place stop: %v — MANUAL ATTENTION NEEDED\n", sig.Ticker, err)
				continue
			}
			_ = buyOrder

			signals = append(signals, sig)
			sharesByTicker[sig.Ticker] = shares
			book.Open(sig.Ticker, sig.EntryPrice, sig.InitialStop, shares, today, stopOrder.ID)

			availableSlots--
			if availableSlots <= 0 {
				break
			}
		}
	}

	fmt.Println("\nFilter breakdown:")
	for stage, count := range stageCounts {
		fmt.Printf("  %s: %d\n", stage, count)
	}

	fmt.Printf("\n%d new signals:\n", len(signals))
	for _, s := range signals {
		fmt.Printf("  %s  entry=%.2f  stop=%.2f  atr=%.2f  rs=%.1f  shares=%d\n",
			s.Ticker, s.EntryPrice, s.InitialStop, s.ATR, s.RSPercentile, sharesByTicker[s.Ticker])
	}

	// ---------- 5. Persist everything ----------
	logger := storage.NewLogger(rt.DataDir + "/live")
	if err := logger.AppendSignals(signals, sharesByTicker); err != nil {
		fmt.Printf("failed to log signals: %v\n", err)
	} else {
		fmt.Println("Signals logged.")
	}

	if err := book.Save(); err != nil {
		fmt.Printf("failed to save position book: %v\n", err)
	} else {
		fmt.Println("Position book saved.")
	}
}
