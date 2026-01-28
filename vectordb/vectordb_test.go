package vectordb

import (
	"testing"
)

func TestDeleteMemories(t *testing.T) {
	q, err := NewQdrantMemoryDB()
	if err != nil {
		t.Error("Got this error while intialising a new qdrant db", "error", err)
	}
	err = q.DeleteMemories([]string{"a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11"}, t.Context())
	if err != nil {
		t.Error("Got this error while deleting memories", "error", err)
	}

}
