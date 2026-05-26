package services

import (
	"encoding/json"
	"net/http"
	"time"
)

type GeoInfo struct {
	CountryCode string `json:"countryCode"`
	Currency    string `json:"currency"`
}

var client = &http.Client{Timeout: 5 * time.Second}

// DetectCurrency gets the country code and returns the appropriate currency
// Default is NGN for Nigeria, GHS for Ghana, USD for others as fallback
func DetectCurrency(ipAddress string) (*GeoInfo, error) {
	// A simple fallback if testing locally
	if ipAddress == "127.0.0.1" || ipAddress == "::1" || ipAddress == "" {
		return &GeoInfo{CountryCode: "NG", Currency: "NGN"}, nil
	}

	url := "http://ip-api.com/json/" + ipAddress
	resp, err := client.Get(url)
	if err != nil {
		return &GeoInfo{CountryCode: "NG", Currency: "NGN"}, nil // fallback
	}
	defer resp.Body.Close()

	var data struct {
		Status      string `json:"status"`
		CountryCode string `json:"countryCode"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return &GeoInfo{CountryCode: "NG", Currency: "NGN"}, nil // fallback
	}

	currency := "NGN"
	if data.CountryCode == "GH" {
		currency = "GHS"
	} else if data.CountryCode != "NG" && data.CountryCode != "" {
		// Just an example fallback for non-supported
		currency = "USD"
	}

	return &GeoInfo{
		CountryCode: data.CountryCode,
		Currency:    currency,
	}, nil
}
