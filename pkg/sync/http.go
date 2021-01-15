package sync

import (
	"encoding/json"
	"fmt"
	"github.com/go-redis/redis/v7"
	"go.uber.org/zap"
	"net/http"
)

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

type Server struct {
	rclient *redis.Client
	log     *zap.SugaredLogger
}

// Publish publishes an item on the supplied topic. The payload type must match
// the payload type on the Topic; otherwise Publish will error.
//
// This method returns synchronously, once the item has been published
// successfully, returning the sequence number of the new item in the ordered
// topic, or an error if one occurred, starting with 1 (for the first item).
//
// If error is non-nil, the sequence number must be disregarded.
func (s *Server) publish(w http.ResponseWriter, r *http.Request) {
	var req PublishRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	log := s.log.With("topic", req.Topic)
	log.Debugw("publishing item on topic", "payload", req.Payload)

	// Serialize the payload.
	bytes, err := json.Marshal(req.Payload)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed while serializing payload: %s", err), http.StatusInternalServerError)
		return
	}

	log.Debugw("serialized json payload", "json", string(bytes))

	// Perform a Redis transaction, adding the item to the stream and fetching
	// the XLEN of the stream.
	args := new(redis.XAddArgs)
	args.ID = "*"
	args.Stream = req.Topic
	args.Values = map[string]interface{}{RedisPayloadKey: bytes}

	pipe := s.rclient.TxPipeline()
	_ = pipe.XAdd(args)
	xlen := pipe.XLen(req.Topic)

	_, err = pipe.ExecContext(r.Context())
	if err != nil {
		log.Debugw("failed to publish item", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	seq := xlen.Val()
	s.log.Debugw("successfully published item; sequence number obtained", "seq", seq)
	s.sendJSON(w, &PublishResponse{Seq: seq})
}

func (s *Server) subscribe(w http.ResponseWriter, r *http.Request) {
	// TODO
}

func (s *Server) barrier(w http.ResponseWriter, r *http.Request) {
	// TODO
}

func (s *Server) signalEntry(w http.ResponseWriter, r *http.Request) {
	var req SignalEntryRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	s.log.Debugw("signalling entry to state", "key", req.State)

	// Increment a counter on the state key.
	seq, err := s.rclient.Incr(req.State).Result()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.log.Debugw("new value of state", "key", req.State, "value", seq)
	s.sendJSON(w, &SignalEntryResponse{Seq: seq})
}

func (s *Server) signalEvent(w http.ResponseWriter, r *http.Request) {
	var req SignalEventRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	ev, err := json.Marshal(req.Event)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	args := &redis.XAddArgs{
		Stream: req.Key,
		ID:     "*",
		Values: map[string]interface{}{RedisPayloadKey: ev},
	}

	_, err = s.rclient.XAdd(args).Result()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
