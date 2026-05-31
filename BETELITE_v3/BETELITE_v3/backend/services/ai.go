package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"time"

	"betelite-go/config"
)

type AIResult struct {
	Winner string `json:"winner"`
	Score1 int    `json:"score1"`
	Score2 int    `json:"score2"`
	Status string `json:"status"`
}

// VerifyMatchResult calls the external AI verification service
// (in this case, the python OCR service running on another port or host)
func VerifyMatchResult(imagePath string) (*AIResult, error) {
	file, err := os.Open(imagePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("image", "match_result.jpg")
	if err != nil {
		return nil, err
	}
	_, err = io.Copy(part, file)
	if err != nil {
		return nil, err
	}
	writer.Close()

	detectionURL := config.Cfg.DetectionServiceURL
	if detectionURL == "" {
		detectionURL = "http://localhost:5000"
	}
	req, err := http.NewRequest("POST", detectionURL+"/api/detect", body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("AI service returned status: %d", resp.StatusCode)
	}

	var res struct {
		Success bool     `json:"success"`
		Result  AIResult `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}

	return &res.Result, nil
}
