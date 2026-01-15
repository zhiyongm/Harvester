package tpe

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const ServerURL = "http://localhost:15000"

type ParamResponse struct {
	K           float64 `json:"k"`
	TrialNumber int     `json:"trial_number"`
}

type ReportRequest struct {
	HitRate float64 `json:"hit_rate"`
}

func InitGetK() (float64, error) {
	resp, err := http.Get(ServerURL + "/init_k")
	if err != nil {
		return 0, fmt.Errorf("failed to connect to python server: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var pResponse ParamResponse
	if err := json.Unmarshal(body, &pResponse); err != nil {
		return 0, fmt.Errorf("failed to parse json: %v", err)
	}

	fmt.Printf("[Go] Initialized with k = %.4f (Trial %d)\n", pResponse.K, pResponse.TrialNumber)
	return pResponse.K, nil
}

func ReportHitRateAndGetNextK(hitRate float64) (float64, error) {
	reqData := ReportRequest{HitRate: hitRate}
	jsonData, _ := json.Marshal(reqData)

	resp, err := http.Post(ServerURL+"/report_and_next", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return 0, fmt.Errorf("failed to report to python server: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var pResponse ParamResponse
	if err := json.Unmarshal(body, &pResponse); err != nil {
		return 0, fmt.Errorf("failed to parse json: %v", err)
	}

	fmt.Printf("[Go] Reported rate %.4f. Received next k = %.4f (Trial %d)\n", hitRate, pResponse.K, pResponse.TrialNumber)
	return pResponse.K, nil
}
