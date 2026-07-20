import pandas as pd
from dataclasses import replace
from config import Strategy
from backtest import run_backtest
from metrics import summarize
from data_loader import load_universe, load_ticker_list


def sweep():
    tickers = load_ticker_list()
    universe = load_universe(tickers)

    results = []
    for rs_min in [70, 75, 80, 85]:
        for trail_mult in [2.5, 3.0, 3.5, 4.0]:
            for time_stop in [4, 6, 8]:
                cfg = replace(Strategy(), rs_percentile_min=rs_min,
                               trail_atr_mult=trail_mult, time_stop_weeks=time_stop)
                trades, final_equity = run_backtest(universe, cfg)
                stats = summarize(trades, 10000.0, final_equity)
                stats.update({"rs_min": rs_min, "trail_mult": trail_mult, "time_stop": time_stop})
                results.append(stats)
                print(stats)

    df = pd.DataFrame(results)
    df.to_csv("results/grid_search_results.csv", index=False)
    return df


if __name__ == "__main__":
    sweep()
