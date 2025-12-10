package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"time"
)

type UploadResult struct {
	Success    bool
	StatusCode int
	Message    string
}

func uploadUsageData(config *Config) (*UploadResult, error) {
	// Collect usage data
	usageData, err := collectUsageData()
	if err != nil {
		return &UploadResult{Success: false, Message: err.Error()}, err
	}

	if len(usageData.Daily) == 0 {
		return &UploadResult{Success: true, Message: "No data to upload"}, nil
	}

	// Convert to JSON
	jsonData, err := json.Marshal(usageData)
	if err != nil {
		return &UploadResult{Success: false, Message: err.Error()}, err
	}

	// Create multipart form
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Add file field
	filePart, err := writer.CreateFormFile("file", "usage.json")
	if err != nil {
		return &UploadResult{Success: false, Message: err.Error()}, err
	}
	filePart.Write(jsonData)

	// Add metadata fields
	hostname, _ := os.Hostname()
	writer.WriteField("hostname", hostname)
	writer.WriteField("timestamp", fmt.Sprintf("%d", time.Now().Unix()))
	writer.WriteField("userEmail", config.Email)

	writer.Close()

	// Send request
	uploadURL := config.ServerURL + "/api/claude-usage/upload"
	req, err := http.NewRequest("POST", uploadURL, &buf)
	if err != nil {
		return &UploadResult{Success: false, Message: err.Error()}, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return &UploadResult{Success: false, Message: err.Error()}, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == 200 || resp.StatusCode == 201 {
		return &UploadResult{
			Success:    true,
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("Uploaded %d days of data", len(usageData.Daily)),
		}, nil
	}

	return &UploadResult{
		Success:    false,
		StatusCode: resp.StatusCode,
		Message:    string(body),
	}, fmt.Errorf("upload failed: HTTP %d", resp.StatusCode)
}
