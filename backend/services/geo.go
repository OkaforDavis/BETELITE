package services

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

type GeoInfo struct {
	CountryCode string `json:"countryCode"`
	Currency    string `json:"currency"`
}

var client = &http.Client{Timeout: 5 * time.Second}

// geoCache keeps IP lookups for an hour (the free lookup service is rate-limited).
var geoCache sync.Map // ip -> geoCached

type geoCached struct {
	info GeoInfo
	at   time.Time
}

// DetectCurrency looks up the country for an IP address and the wallet
// currency used there (NGN, GHS, else USD). When the country can't be found,
// CountryCode is empty and the currency defaults to NGN.
func DetectCurrency(ipAddress string) (*GeoInfo, error) {
	if ipAddress == "127.0.0.1" || ipAddress == "::1" || ipAddress == "" {
		return &GeoInfo{Currency: "NGN"}, nil
	}
	if v, ok := geoCache.Load(ipAddress); ok && time.Since(v.(geoCached).at) < time.Hour {
		info := v.(geoCached).info
		return &info, nil
	}
	unknown := &GeoInfo{Currency: "NGN"}
	resp, err := client.Get("http://ip-api.com/json/" + ipAddress + "?fields=status,countryCode")
	if err != nil {
		return unknown, nil
	}
	defer resp.Body.Close()
	var data struct {
		Status      string `json:"status"`
		CountryCode string `json:"countryCode"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil || data.Status != "success" {
		return unknown, nil
	}
	info := GeoInfo{CountryCode: data.CountryCode, Currency: CurrencyForCountry(data.CountryCode)}
	geoCache.Store(ipAddress, geoCached{info, time.Now()})
	return &info, nil
}

// CurrencyForCountry maps a country to the wallet currency used there.
func CurrencyForCountry(country string) string {
	switch country {
	case "NG", "":
		return "NGN"
	case "GH":
		return "GHS"
	}
	return "USD"
}
