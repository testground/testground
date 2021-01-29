package sync

import (
	"context"
	"errors"
	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
	"time"
)

type connection struct {
	*websocket.Conn
	service   Service
	ctx       context.Context
	responses chan *Response
}

func (c *connection) consumeRequests() error {
	for {
		req, err := c.readTimeout(time.Hour)
		if err != nil {
			return err
		}

		switch {
		case req.PublishRequest != nil:
			go c.publishHandler(req.ID, req.PublishRequest)
		case req.SubscribeRequest != nil:
			go c.subscribeHandler(req.ID, req.SubscribeRequest)
		case req.BarrierRequest != nil:
			go c.barrierHandler(req.ID, req.BarrierRequest)
		case req.SignalEventRequest != nil:
			go c.signalEventHandler(req.ID, req.SignalEventRequest)
		case req.SignalEntryRequest != nil:
			go c.signalEntryHandler(req.ID, req.SignalEntryRequest)
		}
	}
}

func (c *connection) publishHandler(id string, req *PublishRequest) {
	ctx, cancel := context.WithTimeout(c.ctx, time.Second*10)
	defer cancel()

	resp := &Response{ID: id}
	seq, err := c.service.Publish(ctx, req.Topic, req.Payload)
	if err != nil {
		resp.Error = err.Error()
	} else {
		resp.PublishResponse = &PublishResponse{
			Seq: seq,
		}
	}
	c.responses <- resp
}

func (c *connection) subscribeHandler(id string, req *SubscribeRequest) {
	sub, err := c.service.Subscribe(c.ctx, req.Topic)
	if err != nil {
		c.responses <- &Response{ID: id, Error: err.Error()}
		return
	}

	for {
		select {
		case data := <-sub.outCh:
			c.responses <- &Response{ID: id, SubscribeResponse: data}
		case err = <-sub.doneCh:
			if errors.Is(err, context.Canceled) {
				// Cancelled by the user.
				return
			}

			c.responses <- &Response{ID: id, Error: err.Error()}
			return
		case <-c.ctx.Done():
			// Cancelled by the user.
			return
		}
	}
}

func (c *connection) barrierHandler(id string, req *BarrierRequest) {
	resp := &Response{ID: id}
	err := c.service.Barrier(c.ctx, req.State, req.Target)
	if err != nil {
		resp.Error = err.Error()
	}
	c.responses <- resp
}

func (c *connection) signalEntryHandler(id string, req *SignalEntryRequest) {
	ctx, cancel := context.WithTimeout(c.ctx, time.Second*10)
	defer cancel()

	resp := &Response{ID: id}
	seq, err := c.service.SignalEntry(ctx, req.State)
	if err != nil {
		resp.Error = err.Error()
	} else {
		resp.SignalEntryResponse = &SignalEntryResponse{
			Seq: seq,
		}
	}
	c.responses <- resp
}

func (c *connection) signalEventHandler(id string, req *SignalEventRequest) {
	ctx, cancel := context.WithTimeout(c.ctx, time.Second*10)
	defer cancel()

	resp := &Response{ID: id}
	err := c.service.SignalEvent(ctx, req.Key, req.Event)
	if err != nil {
		resp.Error = err.Error()
	}
	c.responses <- resp
}

func (c *connection) consumeResponses() error {
	for {
		select {
		case resp := <-c.responses:
			err := c.writeTimeout(time.Second*10, resp)
			if err != nil {
				return err
			}
		case <-c.ctx.Done():
			return c.ctx.Err()
		}
	}
}

func (c *connection) readTimeout(timeout time.Duration) (*Request, error) {
	ctx, cancel := context.WithTimeout(c.ctx, timeout)
	defer cancel()

	var req *Request
	err := wsjson.Read(ctx, c.Conn, &req)
	return req, err
}

func (c *connection) writeTimeout(timeout time.Duration, resp *Response) error {
	ctx, cancel := context.WithTimeout(c.ctx, timeout)
	defer cancel()
	return wsjson.Write(ctx, c.Conn, resp)
}
