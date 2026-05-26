// Package changelog 修改记录核心模块（FR-3 ★）。
package changelog

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	pmlog "projectManager/internal/log"
	mstore "projectManager/internal/store/mongo"
)

// ChangeLog 修改记录条目。
type ChangeLog struct {
	ID           primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	ProjectID    primitive.ObjectID `bson:"project_id" json:"project_id"`
	FieldKey     string             `bson:"field_key" json:"field_key"`
	FieldLabel   string             `bson:"field_label" json:"field_label"`
	OldValue     interface{}        `bson:"old_value" json:"old_value"`
	NewValue     interface{}        `bson:"new_value" json:"new_value"`
	OperatorID   string             `bson:"operator_id" json:"operator_id"`
	OperatorName string             `bson:"operator_name" json:"operator_name"`
	OperatorRole string             `bson:"operator_role" json:"operator_role"`
	OperatedAt   time.Time          `bson:"operated_at" json:"operated_at"`
	Revision     int32              `bson:"revision" json:"revision"`
	Remark       string             `bson:"remark" json:"remark"`
	Phase        string             `bson:"phase" json:"phase"`
	Hidden       bool               `bson:"hidden" json:"hidden"`
}

// Query 查询条件。
type Query struct {
	ProjectID    string
	FieldKey     string
	OperatorID   string
	OperatorRole string
	From         *time.Time
	To           *time.Time
	Page         int64
	Size         int64
}

// Repo 仓储接口。
type Repo interface {
	InsertMany(ctx context.Context, items []*ChangeLog) error
	List(ctx context.Context, q Query) ([]*ChangeLog, int64, error)
	ListByField(ctx context.Context, projectID, fieldKey string) ([]*ChangeLog, error)
	ListByRevisionRange(ctx context.Context, projectID string, fromRev, toRev int32) ([]*ChangeLog, error)
}

// MongoRepo Mongo 实现。
type MongoRepo struct{ store *mstore.Store }

// NewMongoRepo 构造。
func NewMongoRepo(s *mstore.Store) *MongoRepo { return &MongoRepo{store: s} }

func (r *MongoRepo) coll() *mongo.Collection { return r.store.Coll(mstore.CollProjectChangeLogs) }

// InsertMany 批量插入。
func (r *MongoRepo) InsertMany(ctx context.Context, items []*ChangeLog) error {
	if len(items) == 0 {
		return nil
	}
	docs := make([]interface{}, 0, len(items))
	for _, it := range items {
		if it.ID.IsZero() {
			it.ID = primitive.NewObjectID()
		}
		docs = append(docs, it)
	}
	if _, err := r.coll().InsertMany(ctx, docs); err != nil {
		pmlog.Errorf("changelog InsertMany err=%v count=%d", err, len(items))
		return err
	}
	return nil
}

// List 时间轴列表。
func (r *MongoRepo) List(ctx context.Context, q Query) ([]*ChangeLog, int64, error) {
	pid, err := primitive.ObjectIDFromHex(q.ProjectID)
	if err != nil {
		return nil, 0, errors.New("invalid project_id")
	}
	if q.Page <= 0 {
		q.Page = 1
	}
	if q.Size <= 0 || q.Size > 200 {
		q.Size = 50
	}
	filter := bson.M{"project_id": pid, "hidden": false}
	if q.FieldKey != "" {
		filter["field_key"] = q.FieldKey
	}
	if q.OperatorID != "" {
		filter["operator_id"] = q.OperatorID
	}
	if q.OperatorRole != "" {
		filter["operator_role"] = q.OperatorRole
	}
	if q.From != nil || q.To != nil {
		rng := bson.M{}
		if q.From != nil {
			rng["$gte"] = *q.From
		}
		if q.To != nil {
			rng["$lte"] = *q.To
		}
		filter["operated_at"] = rng
	}
	total, err := r.coll().CountDocuments(ctx, filter)
	if err != nil {
		pmlog.Errorf("changelog Count err=%v", err)
		return nil, 0, err
	}
	opt := options.Find().
		SetSkip((q.Page - 1) * q.Size).
		SetLimit(q.Size).
		SetSort(bson.D{{Key: "revision", Value: -1}, {Key: "operated_at", Value: -1}})
	cur, err := r.coll().Find(ctx, filter, opt)
	if err != nil {
		pmlog.Errorf("changelog Find err=%v", err)
		return nil, 0, err
	}
	defer cur.Close(ctx)
	out := make([]*ChangeLog, 0, q.Size)
	for cur.Next(ctx) {
		var c ChangeLog
		if err := cur.Decode(&c); err != nil {
			pmlog.Errorf("changelog decode err=%v", err)
			return nil, 0, err
		}
		out = append(out, &c)
	}
	return out, total, nil
}

// ListByField 字段历史。
func (r *MongoRepo) ListByField(ctx context.Context, projectID, fieldKey string) ([]*ChangeLog, error) {
	pid, err := primitive.ObjectIDFromHex(projectID)
	if err != nil {
		return nil, errors.New("invalid project_id")
	}
	filter := bson.M{"project_id": pid, "field_key": fieldKey, "hidden": false}
	opt := options.Find().SetSort(bson.D{{Key: "operated_at", Value: -1}})
	cur, err := r.coll().Find(ctx, filter, opt)
	if err != nil {
		pmlog.Errorf("changelog ListByField err=%v", err)
		return nil, err
	}
	defer cur.Close(ctx)
	out := make([]*ChangeLog, 0)
	for cur.Next(ctx) {
		var c ChangeLog
		if err := cur.Decode(&c); err != nil {
			pmlog.Errorf("changelog ListByField decode err=%v", err)
			return nil, err
		}
		out = append(out, &c)
	}
	return out, nil
}

// ListByRevisionRange 取 (fromRev, toRev] 区间的所有日志。
func (r *MongoRepo) ListByRevisionRange(ctx context.Context, projectID string, fromRev, toRev int32) ([]*ChangeLog, error) {
	pid, err := primitive.ObjectIDFromHex(projectID)
	if err != nil {
		return nil, errors.New("invalid project_id")
	}
	filter := bson.M{
		"project_id": pid,
		"hidden":     false,
		"revision":   bson.M{"$gt": fromRev, "$lte": toRev},
	}
	opt := options.Find().SetSort(bson.D{{Key: "revision", Value: 1}})
	cur, err := r.coll().Find(ctx, filter, opt)
	if err != nil {
		pmlog.Errorf("changelog ListByRevisionRange err=%v", err)
		return nil, err
	}
	defer cur.Close(ctx)
	out := make([]*ChangeLog, 0)
	for cur.Next(ctx) {
		var c ChangeLog
		if err := cur.Decode(&c); err != nil {
			pmlog.Errorf("changelog ListByRevisionRange decode err=%v", err)
			return nil, err
		}
		out = append(out, &c)
	}
	return out, nil
}

// Service 修改记录服务。
type Service struct{ repo Repo }

// NewService 构造。
func NewService(r Repo) *Service { return &Service{repo: r} }

// InsertMany 批量插入。
func (s *Service) InsertMany(ctx context.Context, items []*ChangeLog) error {
	return s.repo.InsertMany(ctx, items)
}

// Timeline 时间轴。
func (s *Service) Timeline(ctx context.Context, q Query) ([]*ChangeLog, int64, error) {
	return s.repo.List(ctx, q)
}

// ByField 字段历史。
func (s *Service) ByField(ctx context.Context, projectID, fieldKey string) ([]*ChangeLog, error) {
	return s.repo.ListByField(ctx, projectID, fieldKey)
}

// Diff 取版本区间日志，按字段折叠为最终 diff（旧值=区间起点的旧值，新值=区间最后的新值）。
func (s *Service) Diff(ctx context.Context, projectID string, fromRev, toRev int32) ([]*ChangeLog, error) {
	if fromRev > toRev {
		fromRev, toRev = toRev, fromRev
	}
	logs, err := s.repo.ListByRevisionRange(ctx, projectID, fromRev, toRev)
	if err != nil {
		return nil, err
	}
	first := map[string]*ChangeLog{}
	last := map[string]*ChangeLog{}
	for _, l := range logs {
		if _, ok := first[l.FieldKey]; !ok {
			first[l.FieldKey] = l
		}
		last[l.FieldKey] = l
	}
	out := make([]*ChangeLog, 0, len(first))
	for k, f := range first {
		l := last[k]
		merged := *l
		merged.OldValue = f.OldValue
		out = append(out, &merged)
	}
	return out, nil
}
