package broker

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	KeyID     string
	SecretKey string
	BaseURL   string
	HTTP      *http.Client
}

func NewClient(keyID, secretKey, baseURL string) *Client {
	return &Client{
		KeyID: keyID, SecretKey: secretKey, BaseURL: baseURL,
		HTTP: &http.Client{Timeout: 15 * time.Second},
	}
}

type orderRequest struct {
	Symbol      string `json:"symbol"`
	Qty         string `json:"qty"`
	Side        string `json:"side"`          // "buy" or "sell"
	Type        string `json:"type"`          // "market" or "stop"
	TimeInForce string `json:"time_in_force"` // "day"
	StopPrice   string `json:"stop_price,omitempty"`
}

type OrderResponse struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

func (c *Client) do(method, path string, body interface{}) (*http.Response, error) {
	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reqBody = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, c.BaseURL+path, reqBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("APCA-API-KEY-ID", c.KeyID)
	req.Header.Set("APCA-API-SECRET-KEY", c.SecretKey)
	req.Header.Set("Content-Type", "application/json")
	return c.HTTP.Do(req)
}

func (c *Client) PlaceMarketOrder(symbol string, qty int, side string) (*OrderResponse, error) {
	req := orderRequest{
		Symbol: symbol, Qty: fmt.Sprintf("%d", qty),
		Side: side, Type: "market", TimeInForce: "day",
	}
	resp, err := c.do("POST", "/v2/orders", req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("alpaca order error %d: %s", resp.StatusCode, string(body))
	}
	var out OrderResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) PlaceStopSell(symbol string, qty int, stopPrice float64) (*OrderResponse, error) {
	req := orderRequest{
		Symbol: symbol, Qty: fmt.Sprintf("%d", qty),
		Side: "sell", Type: "stop", TimeInForce: "day",
		StopPrice: fmt.Sprintf("%.2f", stopPrice),
	}
	resp, err := c.do("POST", "/v2/orders", req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("alpaca stop order error %d: %s", resp.StatusCode, string(body))
	}
	var out OrderResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) CancelOrder(orderID string) error {
	resp, err := c.do("DELETE", "/v2/orders/"+orderID, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 && resp.StatusCode != 404 && resp.StatusCode != 422 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("cancel order error %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

func (c *Client) GetOrderStatus(orderID string) (string, error) {
	resp, err := c.do("GET", "/v2/orders/"+orderID, nil)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("get order error %d: %s", resp.StatusCode, string(body))
	}
	var out OrderResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	return out.Status, nil
}
