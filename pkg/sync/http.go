package sync

import (
	"context"
	"encoding/json"
	"errors"
	"go.uber.org/zap"
	"net/http"
)

type Server struct {
	Service
	log *zap.SugaredLogger
}

func NewSyncServer() {
	// TODO: start HTTP server w/ TLS. See how to force HTTP/2 connections.
}

func (s *Server) sendJSON(w http.ResponseWriter, d interface{}) {
	err := json.NewEncoder(w).Encode(d)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		s.log.Errorw("error sending json response", "payload", d)
	}
}

func (s *Server) publish(w http.ResponseWriter, r *http.Request) {
	var req PublishRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	seq, err := s.Publish(r.Context(), req.Topic, req.Payload)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.sendJSON(w, &PublishResponse{Seq: seq})
}

func (s *Server) subscribe(w http.ResponseWriter, r *http.Request) {
	var req SubscribeRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	sub, err := s.Subscribe(r.Context(), req.Topic)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	enc := json.NewEncoder(w)

	for {
		select {
		case data := <-sub.outCh:
			err = enc.Encode(data)
			if err != nil {
				// TODO: handle
				return
			}

			_, err = w.Write([]byte("\n"))
			if err != nil {
				// TODO: handle
				return
			}
		case err = <-sub.doneCh:
			if errors.Is(err, context.Canceled) {
				// Cancelled by the user.
				return
			}
			// TODO: check error
			return
		case <-r.Context().Done():
			// Cancelled by the user.
			return
		}
	}
}

func (s *Server) barrier(w http.ResponseWriter, r *http.Request) {
	var req BarrierRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	err = s.Barrier(r.Context(), req.State, req.Target)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (s *Server) signalEntry(w http.ResponseWriter, r *http.Request) {
	var req SignalEntryRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	seq, err := s.SignalEntry(r.Context(), req.State)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.sendJSON(w, &SignalEntryResponse{Seq: seq})
}

func (s *Server) signalEvent(w http.ResponseWriter, r *http.Request) {
	var req SignalEventRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	err = s.SignalEvent(r.Context(), req.Key, req.Event)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
