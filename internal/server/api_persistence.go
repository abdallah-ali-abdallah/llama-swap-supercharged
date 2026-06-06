package server

import (
	"encoding/json"
	"net/http"
)

// handleAPIPersistenceSettings returns the current persistence settings.
func (s *Server) handleAPIPersistenceSettings(w http.ResponseWriter, r *http.Request) {
	if s.metrics == nil || s.metrics.store == nil {
		http.Error(w, `{"error":"persistence not available"}`, http.StatusServiceUnavailable)
		return
	}
	ps := s.metrics.store.getSettings()
	ps.YAMLAvailable = s.cfg.ConfigPath != ""
	ps.YAMLPath = s.cfg.ConfigPath
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ps)
}

// handleAPIUpdatePersistenceSettings updates persistence settings.
func (s *Server) handleAPIUpdatePersistenceSettings(w http.ResponseWriter, r *http.Request) {
	if s.metrics == nil || s.metrics.store == nil {
		http.Error(w, `{"error":"persistence not available"}`, http.StatusServiceUnavailable)
		return
	}
	var ps persistenceSettings
	if err := json.NewDecoder(r.Body).Decode(&ps); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if err := s.metrics.store.updateSettings(ps); err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s.metrics.store.getSettings())
}
