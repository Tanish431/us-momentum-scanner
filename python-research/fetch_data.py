import os
import time
import requests
import pandas as pd
from pathlib import Path
from datetime import datetime, timedelta

ALPACA_KEY = os.environ["ALPACA_KEY_ID"]
ALPACA_SECRET = os.environ["ALPACA_SECRET_KEY"]
BASE_URL = "https://data.alpaca.markets/v2/stocks"

HEADERS = {
    "APCA-API-KEY-ID": ALPACA_KEY,
    "APCA-API-SECRET-KEY": ALPACA_SECRET,
}


def fetch_bars(ticker: str, start: str, end: str, timeframe: str = "1Day") -> pd.DataFrame:
    """Pulls daily bars for one ticker, handling pagination via next_page_token.
    IEX feed is the free tier — pass feed=iex explicitly, default may require paid SIP."""
    url = f"{BASE_URL}/{ticker}/bars"
    params = {
        "start": start,
        "end": end,
        "timeframe": timeframe,
        "limit": 10000,
        "feed": "iex",
        "adjustment": "split",  # adjust for splits; use "all" if you also want dividend adj
    }

    all_bars = []
    page_token = None
    while True:
        if page_token:
            params["page_token"] = page_token
        resp = requests.get(url, headers=HEADERS, params=params)
        if resp.status_code == 429:
            print(f"  rate limited on {ticker}, sleeping 30s")
            time.sleep(30)
            continue
        resp.raise_for_status()
        data = resp.json()
        bars = data.get("bars", [])
        all_bars.extend(bars)
        page_token = data.get("next_page_token")
        if not page_token:
            break

    if not all_bars:
        return pd.DataFrame()

    df = pd.DataFrame(all_bars)
    df = df.rename(columns={"t": "date", "o": "open", "h": "high", "l": "low", "c": "close", "v": "volume"})
    df["date"] = pd.to_datetime(df["date"]).dt.tz_localize(None).dt.normalize()
    return df[["date", "open", "high", "low", "close", "volume"]]


def fetch_and_save(ticker: str, start: str, end: str, out_dir: str = "data/historical"):
    Path(out_dir).mkdir(parents=True, exist_ok=True)
    out_path = Path(out_dir) / f"{ticker}.csv"
    try:
        df = fetch_bars(ticker, start, end)
        if df.empty:
            print(f"[empty] {ticker}")
            return
        df.to_csv(out_path, index=False)
        print(f"[ok] {ticker}: {len(df)} bars")
    except requests.HTTPError as e:
        print(f"[fail] {ticker}: {e}")


def fetch_universe(tickers: list[str], start: str, end: str, out_dir: str = "data/historical"):
    for i, ticker in enumerate(tickers):
        fetch_and_save(ticker, start, end, out_dir)
        # free tier rate limit is 200 req/min; stay well under it
        time.sleep(0.35)
        if (i + 1) % 50 == 0:
            print(f"-- {i + 1}/{len(tickers)} done --")


if __name__ == "__main__":
    from data_loader import load_ticker_list

    tickers = load_ticker_list()  # data/universe/sp500.csv
    end = datetime.today().strftime("%Y-%m-%d")
    start = (datetime.today() - timedelta(days=5 * 365)).strftime("%Y-%m-%d")  # 5 years back

    fetch_universe(tickers, start, end)
