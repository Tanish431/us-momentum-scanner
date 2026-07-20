from dataclasses import dataclass

@dataclass
class Strategy:
    breakout_lookback_days: int = 20
    volume_avg_days: int = 50
    volume_multiplier: float = 1.5
    rs_lookback_days: int = 63
    rs_percentile_min: float = 80.0
    trend_fast_sma: int = 50
    trend_slow_sma: int = 200
    atr_period: int = 14
    atr_pct_skip_decline: float = 0.90
    initial_stop_atr_mult: float = 2.0
    trail_atr_mult: float = 3.5
    time_stop_weeks: int = 6
    risk_per_trade_pct: float = 0.75
    max_concurrent_pos: int = 5

LOCKED = Strategy()
