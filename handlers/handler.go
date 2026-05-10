package handlers

import (
	"dataverification/common"
	dao "dataverification/dao"
	"net/http"
)

type Handler struct {
	Service dao.MongoDBInstance
}

func NewHandlers(service dao.MongoDBInstance) *Handler {
	return &Handler{Service: service}
}
func (h *Handler) VerifyConnectedSellers(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	startDate := common.ParseDate(query.Get("start_date"))
	endDate := common.ParseDate(query.Get("end_date"))
	err := h.Service.VerifySellers(startDate, endDate)
	if err != nil {
		http.Error(w, "Failed to verify the  seller ingestion", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Data Verified Successfully"))
}
