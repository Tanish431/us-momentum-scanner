import pandas as pd
import numpy as np
from dataclasses import dataclass, field
from config import Strategy
import indicators as ind


@dataclass
class Trade:
    ticker: str
    entry_date: pd.Timestamp
    entry_price: float
    initial_stop: float
    shares: int
    exit_date: pd.Timestamp = None
    exit_price: float = None
    exit_reason: str = None

    @property
    def r_multiple(self) -> float:
        risk_per_share = self.entry_price - self.initial_stop
        if self.exit_price is None or risk_per_share <= 0:
            return np.nan
        return (self.exit_price - self.entry_price) / risk_per_share

    @property
    def pnl(self) -> float:
        if self.exit_price is None:
            return np.nan
        return (self.exit_price - self.entry_price) * self.shares


def precompute(df: pd.DataFrame, cfg: Strategy) -> pd.DataFrame:
    df = df.copy()
    df["sma_fast"] = ind.sma(df["close"], cfg.trend_fast_sma)
    df["sma_slow"] = ind.sma(df["close"], cfg.trend_slow_sma)
    df["donchian_high"] = ind.donchian_high(df["high"], cfg.breakout_lookback_days)
    df["avg_vol"] = ind.avg_volume(df["volume"], cfg.volume_avg_days).shift(1)
    df["atr"] = ind.atr(df, cfg.atr_period)
    df["atr_pct"] = ind.atr_pct(df, cfg.atr_period)
    df["roc"] = ind.rate_of_change(df["close"], cfg.rs_lookback_days)
    return df


def build_rs_percentiles(universe: dict[str, pd.DataFrame], dates: pd.DatetimeIndex) -> pd.DataFrame:
    """Cross-sectional RS percentile per date. Returns date x ticker matrix."""
    roc_matrix = pd.DataFrame({t: df.set_index("date")["roc"] for t, df in universe.items()})
    roc_matrix = roc_matrix.reindex(dates)
    return roc_matrix.rank(axis=1, pct=True) * 100


def run_backtest(universe: dict[str, pd.DataFrame], cfg: Strategy, starting_equity: float = 10000.0):
    universe = {t: precompute(df, cfg) for t, df in universe.items()}
    all_dates = sorted(set().union(*[set(df["date"]) for df in universe.values()]))
    all_dates = pd.DatetimeIndex(all_dates)

    rs_pct = build_rs_percentiles(universe, all_dates)
    atr_pct_matrix = pd.DataFrame({t: df.set_index("date")["atr_pct"] for t, df in universe.items()}).reindex(all_dates)
    atr_skip_threshold = atr_pct_matrix.quantile(cfg.atr_pct_skip_decline, axis=1)

    indexed = {t: df.set_index("date") for t, df in universe.items()}

    equity = starting_equity
    open_positions: dict[str, Trade] = {}
    trailing_stops: dict[str, float] = {}
    weeks_open: dict[str, int] = {}
    closed_trades: list[Trade] = []

    for date in all_dates:
        # --- manage open positions first: check stops, trail, time-stop ---
        for ticker in list(open_positions.keys()):
            if date not in indexed[ticker].index:
                continue
            row = indexed[ticker].loc[date]
            trade = open_positions[ticker]
            stop = trailing_stops[ticker]

            # stopped out intraday (using Low to check)
            if row["low"] <= stop:
                trade.exit_date = date
                trade.exit_price = min(stop, row["open"])  # gap-through handling
                trade.exit_reason = "stop"
                closed_trades.append(trade)
                del open_positions[ticker]
                del trailing_stops[ticker]
                del weeks_open[ticker]
                equity += trade.pnl
                continue

            # ratchet trail upward only
            new_stop = row["close"] - row["atr"] * cfg.trail_atr_mult
            if new_stop > stop:
                trailing_stops[ticker] = new_stop

            # time stop: no meaningful progress within N weeks
            if date.weekday() == 4:  # Friday, weekly check
                weeks_open[ticker] += 1
                if weeks_open[ticker] >= cfg.time_stop_weeks and row["close"] <= trade.entry_price:
                    trade.exit_date = date
                    trade.exit_price = row["close"]
                    trade.exit_reason = "time_stop"
                    closed_trades.append(trade)
                    equity += trade.pnl
                    del open_positions[ticker]
                    del trailing_stops[ticker]
                    del weeks_open[ticker]

        # --- look for new entries if slots available ---
        if len(open_positions) >= cfg.max_concurrent_pos:
            continue

        for ticker, df in indexed.items():
            if ticker in open_positions or date not in df.index:
                continue
            row = df.loc[date]
            if pd.isna(row.get("donchian_high")) or pd.isna(row.get("atr")):
                continue

            rs_val = rs_pct.loc[date, ticker] if ticker in rs_pct.columns else np.nan
            atr_skip = atr_skip_threshold.loc[date] if date in atr_skip_threshold.index else np.nan

            passes = (
                row["close"] > row["donchian_high"]
                and row["volume"] >= row["avg_vol"] * cfg.volume_multiplier
                and not pd.isna(rs_val) and rs_val >= cfg.rs_percentile_min
                and row["close"] > row["sma_fast"] > row["sma_slow"]
                and not pd.isna(atr_skip) and row["atr_pct"] < atr_skip
            )
            if not passes:
                continue

            entry_price = row["close"]
            stop = entry_price - row["atr"] * cfg.initial_stop_atr_mult
            risk_amount = equity * (cfg.risk_per_trade_pct / 100)
            shares = int(risk_amount / (entry_price - stop)) if entry_price > stop else 0
            if shares <= 0:
                continue

            trade = Trade(ticker, date, entry_price, stop, shares)
            open_positions[ticker] = trade
            trailing_stops[ticker] = stop
            weeks_open[ticker] = 0

            if len(open_positions) >= cfg.max_concurrent_pos:
                break

    return closed_trades, equity
