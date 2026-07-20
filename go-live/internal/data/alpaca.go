package data

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/Tanish431/us-momentum-scanner/internal/indicators"
)

type Client struct {
	keyID     string
	SecretKey string
	BaseURL   string
	HTTP      *http.Client
}

func NewClient(keyID, secretKey, baseURL string) *Client {
	return &Client{
		keyID:     keyID,
		SecretKey: secretKey,
		BaseURL:   baseURL,
		HTTP:      http.DefaultClient,
	}
}

type barsResponse struct {
	Bars          []rawBar `json:"bars"`
	NextPageToken *string  `json:"next_page_token"`
}

type rawBar struct {
	T string  `json:"t"`
	O float64 `json:"o"`
	H float64 `json:"h"`
	L float64 `json:"l"`
	C float64 `json:"c"`
	V float64 `json:"v"`
}

func (c *Client) GetDailyBars(ticker, start, end string) ([]indicators.Bar, error) {
	var allBars []indicators.Bar
	pageToken := ""

	for {
		u := fmt.Sprintf("%s/stocks/%s/bars", c.BaseURL, ticker)
		q := url.Values{}
		q.Set("start", start)
		q.Set("end", end)
		q.Set("timeframe", "1Day")
		q.Set("limit", "10000")
		q.Set("feed", "iex")
		q.Set("adjustment", "split")
		if pageToken != "" {
			q.Set("page_token", pageToken)
		}

		req, err := http.NewRequest("GET", u+"?"+q.Encode(), nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("APCA-API-KEY-ID", c.keyID)
		req.Header.Set("APCA-API-SECRET-KEY", c.SecretKey)

		resp, err := c.HTTP.Do(req)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode == 429 {
			resp.Body.Close()
			time.Sleep(30 * time.Second)
			continue
		}
		if resp.StatusCode != 200 {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("alpaca error %d for %s: %s", resp.StatusCode, ticker, string(body))
		}

		var parsed barsResponse
		if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
			resp.Body.Close()
			return nil, err
		}
		resp.Body.Close()

		for _, rb := range parsed.Bars {
			t, err := time.Parse(time.RFC3339, rb.T)
			if err != nil {
				continue
			}
			allBars = append(allBars, indicators.Bar{
				Date:   t.Format("2006-01-02"),
				Open:   rb.O,
				High:   rb.H,
				Low:    rb.L,
				Close:  rb.C,
				Volume: rb.V,
			})
		}
		if parsed.NextPageToken == nil || *parsed.NextPageToken == "" {
			break
		}
		pageToken = *parsed.NextPageToken
		time.Sleep(350 * time.Millisecond)
	}
	return allBars, nil
}

func (c *Client) GetUniverseBars(tickers []string, start, end string) map[string][]indicators.Bar {
	result := make(map[string][]indicators.Bar, len(tickers))
	for i, ticker := range tickers {
		bars, err := c.GetDailyBars(ticker, start, end)
		if err != nil {
			fmt.Printf("[skip] %s: %v\n", ticker, err)
			continue
		}
		if len(bars) == 0 {
			continue
		}
		result[ticker] = bars
		time.Sleep(350 * time.Millisecond)
		if (i+1)%50 == 0 {
			fmt.Printf("-- fetched %d/%d --\n", i+1, len(tickers))
		}
	}
	return result
}
