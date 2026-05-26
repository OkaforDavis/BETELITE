package services

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"betelite-go/config"
)

type FixtureData struct {
	Fixture struct {
		ID        int    `json:"id"`
		Date      string `json:"date"`
		Status    struct {
			Short string `json:"short"`
		} `json:"status"`
	} `json:"fixture"`
	Teams struct {
		Home struct {
			Name string `json:"name"`
			Logo string `json:"logo"`
		} `json:"home"`
		Away struct {
			Name string `json:"name"`
			Logo string `json:"logo"`
		} `json:"away"`
	} `json:"teams"`
	Goals struct {
		Home int `json:"home"`
		Away int `json:"away"`
	} `json:"goals"`
}

// FetchLiveFixtures fetches live football matches from API-Football
func FetchLiveFixtures() ([]FixtureData, error) {
	if config.Cfg.APIFootballKey == "" {
		return nil, fmt.Errorf("Football API Key not configured")
	}

	url := "https://v3.football.api-sports.io/fixtures?live=all"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-apisports-key", config.Cfg.APIFootballKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var res struct {
		Response []FixtureData `json:"response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}

	return res.Response, nil
}
