package sync

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"github.com/gorilla/mux"
	"github.com/testground/testground/pkg/logging"
	"go.uber.org/zap"
	"net"
	"net/http"
)

type Server struct {
	Service
	server *http.Server
	l      net.Listener
	log    *zap.SugaredLogger
}

func NewSyncServer(ctx context.Context, cfg *RedisConfiguration) (srv *Server, err error) {
	srv = new(Server)
	srv.log = logging.S()
	srv.Service, err = NewService(ctx, srv.log, cfg)
	if err != nil {
		return nil, err
	}

	r := mux.NewRouter().StrictSlash(true)

	r.HandleFunc("/publish", srv.publish).Methods("POST")
	r.HandleFunc("/subscribe", srv.subscribe).Methods("POST")
	r.HandleFunc("/barrier", srv.barrier).Methods("POST")
	r.HandleFunc("/signal/event", srv.signalEvent).Methods("POST")
	r.HandleFunc("/signal/entry", srv.signalEntry).Methods("POST")

	srv.server = &http.Server{
		Handler: r,
		TLSConfig: &tls.Config{
			NextProtos: []string{"h2"},
		},
	}

	srv.l, err = net.Listen("tcp", ":443")
	if err != nil {
		return nil, err
	}

	return srv, nil
}

func (s *Server) Serve() error {
	s.log.Infow("daemon listening", "addr", s.Addr())
	return s.server.ServeTLS(s.l, "certs/localhost.crt", "certs/localhost.key")
}

func (s *Server) Addr() string {
	return s.l.Addr().String()
}

func (s *Server) Port() int {
	return s.l.Addr().(*net.TCPAddr).Port
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
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
