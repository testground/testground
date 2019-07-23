package store

import "testing"

func TestSqlite3StoreInitSmokeTest(t *testing.T) {
	if _, err := NewSqliteStore(); err != nil {
		t.Fatal(err)
	}
}
