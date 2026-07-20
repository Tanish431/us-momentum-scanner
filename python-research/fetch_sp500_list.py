import pandas as pd
import requests
from pathlib import Path
from io import StringIO

def build_sp500_list():
    url = "https://en.wikipedia.org/wiki/List_of_S%26P_500_companies"
    headers = {"User-Agent": "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36"}

    resp = requests.get(url, headers=headers)
    resp.raise_for_status()

    tables = pd.read_html(StringIO(resp.text))
    df = tables[0][["Symbol"]].rename(columns={"Symbol": "ticker"})
    df["ticker"] = df["ticker"].str.replace(".", "-", regex=False)  # Alpaca uses BRK-B not BRK.B

    Path("data/universe").mkdir(parents=True, exist_ok=True)
    df.to_csv("data/universe/sp500.csv", index=False)
    print(f"saved {len(df)} tickers")

if __name__ == "__main__":
    build_sp500_list()
