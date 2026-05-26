package changelog

import (
	"context"
	"sync"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type memRepo struct {
	mu   sync.Mutex
	data []*ChangeLog
}

func (r *memRepo) InsertMany(_ context.Context, items []*ChangeLog) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, it := range items {
		if it.ID.IsZero() {
			it.ID = primitive.NewObjectID()
		}
		r.data = append(r.data, it)
	}
	return nil
}
func (r *memRepo) List(_ context.Context, q Query) ([]*ChangeLog, int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]*ChangeLog, 0)
	pid, _ := primitive.ObjectIDFromHex(q.ProjectID)
	for _, l := range r.data {
		if l.ProjectID != pid {
			continue
		}
		out = append(out, l)
	}
	return out, int64(len(out)), nil
}
func (r *memRepo) ListByField(_ context.Context, projectID, fieldKey string) ([]*ChangeLog, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	pid, _ := primitive.ObjectIDFromHex(projectID)
	out := make([]*ChangeLog, 0)
	for _, l := range r.data {
		if l.ProjectID == pid && l.FieldKey == fieldKey {
			out = append(out, l)
		}
	}
	return out, nil
}
func (r *memRepo) ListByRevisionRange(_ context.Context, projectID string, from, to int32) ([]*ChangeLog, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	pid, _ := primitive.ObjectIDFromHex(projectID)
	out := make([]*ChangeLog, 0)
	for _, l := range r.data {
		if l.ProjectID == pid && l.Revision > from && l.Revision <= to {
			out = append(out, l)
		}
	}
	return out, nil
}

// Diff 应折叠区间内同字段的多次变更，保留最早的旧值与最后的新值。
func TestDiff_FoldFieldChanges(t *testing.T) {
	r := &memRepo{}
	pid := primitive.NewObjectID()
	now := time.Now()
	r.data = []*ChangeLog{
		{ID: primitive.NewObjectID(), ProjectID: pid, FieldKey: "f1", OldValue: "a", NewValue: "b", Revision: 2, OperatedAt: now},
		{ID: primitive.NewObjectID(), ProjectID: pid, FieldKey: "f1", OldValue: "b", NewValue: "c", Revision: 3, OperatedAt: now.Add(time.Second)},
		{ID: primitive.NewObjectID(), ProjectID: pid, FieldKey: "f2", OldValue: nil, NewValue: 10, Revision: 3, OperatedAt: now},
	}
	s := NewService(r)
	out, err := s.Diff(context.Background(), pid.Hex(), 1, 3)
	if err != nil {
		t.Fatalf("diff err=%v", err)
	}
	if len(out) != 2 {
		t.Fatalf("expect 2, got %d", len(out))
	}
	for _, l := range out {
		if l.FieldKey == "f1" {
			if l.OldValue != "a" || l.NewValue != "c" {
				t.Fatalf("f1 fold wrong old=%v new=%v", l.OldValue, l.NewValue)
			}
		}
	}
}
