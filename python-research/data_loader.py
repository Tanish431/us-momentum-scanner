import pandas as pd
from pathlib import Path

def load_symbol(ticker: str, data_dir: str = "data/historical") -> pd.DataFrame:
    path = Path(data_dir) / f"{ticker}.csv"
    df = pd.read_csv(path, parse_dates=["date"])
    df = df.sort_values("date").reset_index(drop=True)
    df["ticker"] = ticker
    return df

def load_universe(tickers: list[str], data_dir: str = "data/historical") -> dict[str, pd.DataFrame]:
    universe = {}
    for t in tickers:
        try:
            universe[t] = load_symbol(t, data_dir)
        except FileNotFoundError:
            print(f"[skip] no data for {t}")
    return universe


def load_ticker_list(path: str = "data/universe/sp500.csv") -> list[str]:
    return pd.read_csv(path)["ticker"].tolist()
