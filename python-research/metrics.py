import numpy as np
import pandas as pd
from backtest import Trade


def summarize(trades: list[Trade], starting_equity: float, final_equity: float) -> dict:
    r_multiples = [t.r_multiple for t in trades if not np.isnan(t.r_multiple)]
    wins = [r for r in r_multiples if r > 0]
    losses = [r for r in r_multiples if r <= 0]

    equity_curve = starting_equity + np.cumsum([t.pnl for t in trades if not np.isnan(t.pnl)])
    running_max = np.maximum.accumulate(equity_curve) if len(equity_curve) else np.array([starting_equity])
    drawdown = (equity_curve - running_max) / running_max if len(equity_curve) else np.array([0])

    return {
        "total_trades": len(trades),
        "win_rate_pct": round(len(wins) / len(r_multiples) * 100, 2) if r_multiples else 0,
        "avg_r_win": round(np.mean(wins), 2) if wins else 0,
        "avg_r_loss": round(np.mean(losses), 2) if losses else 0,
        "expectancy_r": round(np.mean(r_multiples), 2) if r_multiples else 0,
        "max_drawdown_pct": round(drawdown.min() * 100, 2) if len(drawdown) else 0,
        "final_equity": round(final_equity, 2),
        "total_return_pct": round((final_equity - starting_equity) / starting_equity * 100, 2),
        "biggest_winner_r": round(max(r_multiples), 2) if r_multiples else 0,
        "biggest_loser_r": round(min(r_multiples), 2) if r_multiples else 0,
    }
