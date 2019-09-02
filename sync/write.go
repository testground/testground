package sync

import (
	"encoding/json"
	"fmt"

	"reflect"

	capi "github.com/hashicorp/consul/api"
	"github.com/ipfs/testground/api"
)

// Writer perform writes on this test run's sync tree.
type Writer struct {
	client *capi.Client
	sid    string // sid is the session id.
	root   string
	doneCh chan struct{}
}

// TODO add a mode to prevent watches firing on keys added from a writer.

// NewWriter creates a new Writer for a particular test run.
func NewWriter(client *capi.Client, runenv *api.RunEnv) (w *Writer, err error) {
	opts := &capi.SessionEntry{
		TTL:      "10s",
		Behavior: "delete",
	}

	sid, _, err := client.Session().Create(opts, nil)
	if err != nil {
		return nil, err
	}

	w = &Writer{
		client: client,
		sid:    sid,
		root:   basePrefix(runenv),
		doneCh: make(chan struct{}),
	}

	// Launch the periodic session renewer.
	go client.Session().RenewPeriodic("3s", sid, nil, w.doneCh)

	return w, nil
}

// Write writes a payload in a subtree. It panics if the payload's type does
// not match the expected type for the subtree. If the actual write on the sync
// service fails, this method returns an error.
func (w *Writer) Write(subtree *Subtree, payload interface{}) (err error) {
	subtree.AssertType(reflect.ValueOf(payload).Type())
	bytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	key := fmt.Sprintf("%s/%s", w.root, subtree.PathFunc(payload))
	kv := &capi.KVPair{
		Key:   key,
		Value: bytes,
	}

	_, err = w.client.KV().Put(kv, nil)
	return err
}

// Close closes this writer and destroys the sync session.
func (w *Writer) Close() error {
	close(w.doneCh)
	_, err := w.client.Session().Destroy(w.sid, nil)
	return err
}
