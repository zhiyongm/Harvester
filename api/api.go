package api

import (
	"blockcooker/chain"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	log "github.com/sirupsen/logrus"
)

type Metrics struct {
	sync.Mutex

	RequestCount  uint64
	TotalDuration time.Duration

	CumulativeRequestCount  uint64
	CumulativeTotalDuration time.Duration
}

func (m *Metrics) RecordCall(duration time.Duration) {
	m.Lock()
	defer m.Unlock()

	m.RequestCount++
	m.TotalDuration += duration

	m.CumulativeRequestCount++
	m.CumulativeTotalDuration += duration
}

func (m *Metrics) ReportAndReset() (count uint64, avgDuration time.Duration) {
	m.Lock()
	defer m.Unlock()

	count = m.RequestCount
	if count == 0 {
		return 0, 0
	}

	avgDuration = m.TotalDuration / time.Duration(count)

	m.RequestCount = 0
	m.TotalDuration = 0

	return count, avgDuration
}

func (m *Metrics) ReportCumulative() (count uint64, avgDuration time.Duration) {
	m.Lock()
	defer m.Unlock()

	count = m.CumulativeRequestCount
	if count == 0 {
		return 0, 0
	}

	avgDuration = m.CumulativeTotalDuration / time.Duration(count)
	return count, avgDuration
}

type API struct {
	bc      *chain.BlockChain
	metrics *Metrics
}

type JsonResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

func NewAPI(bc *chain.BlockChain) *API {
	return &API{
		bc:      bc,
		metrics: &Metrics{},
	}
}

func writeJSON(w http.ResponseWriter, statusCode int, success bool, data interface{}, errMessage string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	response := JsonResponse{
		Success: success,
		Data:    data,
		Error:   errMessage,
	}
	json.NewEncoder(w).Encode(response)
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
	})
}

func (a *API) startMetricsReporter() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		count, avgDuration := a.metrics.ReportAndReset()
		if count > 0 {
			log.Infof(
				"⚡️ Instant Metrics: Calls=%d, Avg Duration=%.2fms",
				count,
				float64(avgDuration.Microseconds())/1000.0,
			)
		} else {
			log.Debug("Instant Metrics: No calls in the last second.")
		}

		cumCount, cumAvgDuration := a.metrics.ReportCumulative()
		if cumCount > 0 {
			log.Infof(
				"📈 Cumulative Metrics: Total Calls=%d, Overall Avg Duration=%.2fms",
				cumCount,
				float64(cumAvgDuration.Microseconds())/1000.0,
			)
		}
	}
}

func (a *API) Start(port int) {

	mux := http.NewServeMux()
	mux.HandleFunc("/account", a.getAccountState)

	loggedMux := loggingMiddleware(mux)

	log.Infof("API server starting on port %d", port)
	err := http.ListenAndServe(":"+strconv.Itoa(port), loggedMux)
	if err != nil {
		log.Fatalf("API server failed to start: %v", err)
	}
}

func (a *API) getAccountState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, false, nil, "Method not allowed")
		return
	}

	addressHex := r.URL.Query().Get("address")
	if addressHex == "" {
		writeJSON(w, http.StatusBadRequest, false, nil, "address parameter is missing")
		return
	}

	address := common.HexToAddress(addressHex)

	start := time.Now()

	account, exist := a.bc.GetAccountState_for_API(address)

	duration := time.Since(start)
	a.metrics.RecordCall(duration)

	if !exist {
		writeJSON(w, http.StatusNotFound, false, nil, fmt.Sprintf("account not found for address: %s", addressHex))
		return
	}

	writeJSON(w, http.StatusOK, true, account, "")
}
