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

	go func() {
		if err := h.Service.VerifySellers(startDate, endDate); err != nil {
			common.SendSlackMessage("Sales data verification FAILED: " + err.Error())
			return
		}
		common.SendSlackMessage("Sales data verification completed. Findings written to the CSV.")
	}()

	w.WriteHeader(http.StatusAccepted)
	w.Write([]byte("Verification started. Findings will be written to the CSV and notified on Slack."))
}
