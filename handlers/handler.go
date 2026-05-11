package handlers

import (
	"dataverification/common"
	dao "dataverification/dao"
	"net/http"
	"time"
)

type Handler struct {
	Service dao.MongoDBInstance
}

func NewHandlers(service dao.MongoDBInstance) *Handler {
	return &Handler{Service: service}
}
func (h *Handler) VerifyConnectedSellers(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	startStr := query.Get("start_date")
	endStr := query.Get("end_date")

	var startDate, endDate time.Time
	if startStr == "" && endStr == "" {
		now := time.Now()
		todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		startDate = todayStart.AddDate(0, 0, -1)
		endDate = todayStart
	} else {
		startDate = common.ParseDate(startStr)
		endDate = common.ParseDate(endStr).AddDate(0, 0, 1)
	}

	err := h.Service.VerifySellers(startDate, endDate)
	if err != nil {
		http.Error(w, "Failed to verify the  seller ingestion", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Data Verified Successfully"))
}
