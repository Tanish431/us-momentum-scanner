import pandas as pd
import numpy as np


def sma(close: pd.Series, period: int) -> pd.Series:
    return close.rolling(period).mean()


def avg_volume(volume: pd.Series, period: int) -> pd.Series:
    return volume.rolling(period).mean()


def donchian_high(high: pd.Series, period: int) -> pd.Series:
    """Highest High over the prior `period` bars, excluding today —
    shift(1) first so today's breakout compares against yesterday's window."""
    return high.shift(1).rolling(period).max()


def atr(df: pd.DataFrame, period: int) -> pd.Series:
    prev_close = df["close"].shift(1)
    tr = pd.concat([
        df["high"] - df["low"],
        (df["high"] - prev_close).abs(),
        (df["low"] - prev_close).abs(),
    ], axis=1).max(axis=1)
    return tr.ewm(alpha=1 / period, adjust=False, min_periods=period).mean()


def atr_pct(df: pd.DataFrame, period: int) -> pd.Series:
    return atr(df, period) / df["close"]


def rate_of_change(close: pd.Series, period: int) -> pd.Series:
    return (close - close.shift(period)) / close.shift(period) * 100
