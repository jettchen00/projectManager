package project

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

// Repo 项目存储接口。
type Repo interface {
	Insert(ctx context.Context, p *Project) error
	GetByID(ctx context.Context, id string) (*Project, error)
	List(ctx context.Context, q ListQuery) ([]*Project, int64, error)
	UpdateStatus(ctx context.Context, id, fromStatus, toStatus string, extra map[string]interface{}) error
	IncRevision(ctx context.Context, id string) (int32, error)
	DecRevision(ctx context.Context, id string) error
	UpdateLastPhase(ctx context.Context, id, phase string) error
	ExistsByNameAndOwnerActive(ctx context.Context, name, ownerName string) (bool, error)
}

// ListQuery 列表查询参数。
type ListQuery struct {
	Status      string
	Keyword     string
	OwnerID     string
	ApplicantID string
	Page        int64
	Size        int64
}

// MongoRepo MongoDB 实现。
type MongoRepo struct{ store *mstore.Store }

// NewMongoRepo 构造。
func NewMongoRepo(s *mstore.Store) *MongoRepo { return &MongoRepo{store: s} }

func (r *MongoRepo) coll() *mongo.Collection { return r.store.Coll(mstore.CollProjects) }

// Insert 插入项目。
func (r *MongoRepo) Insert(ctx context.Context, p *Project) error {
	if p.ID.IsZero() {
		p.ID = primitive.NewObjectID()
	}
	now := time.Now()
	if p.CreatedAt.IsZero() {
		p.CreatedAt = now
	}
	p.UpdatedAt = now
	if _, err := r.coll().InsertOne(ctx, p); err != nil {
		pmlog.Errorf("project Insert err=%v code=%s", err, p.ProjectCode)
		return err
	}
	return nil
}

// GetByID 按 id 取项目。
func (r *MongoRepo) GetByID(ctx context.Context, id string) (*Project, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, nil
	}
	var p Project
	err = r.coll().FindOne(ctx, bson.M{"_id": oid}).Decode(&p)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		pmlog.Errorf("project GetByID err=%v id=%s", err, id)
		return nil, err
	}
	return &p, nil
}

// List 列表 + 总数。
func (r *MongoRepo) List(ctx context.Context, q ListQuery) ([]*Project, int64, error) {
	if q.Page <= 0 {
		q.Page = 1
	}
	if q.Size <= 0 || q.Size > 100 {
		q.Size = 20
	}
	filter := bson.M{}
	if q.Status != "" {
		filter["status"] = q.Status
	}
	if q.OwnerID != "" {
		if oid, err := primitive.ObjectIDFromHex(q.OwnerID); err == nil {
			filter["owner_id"] = oid
		}
	}
	if q.ApplicantID != "" {
		filter["applicant_id"] = q.ApplicantID
	}
	if q.Keyword != "" {
		filter["$or"] = bson.A{
			bson.M{"project_name": bson.M{"$regex": primitive.Regex{Pattern: regexEscape(q.Keyword), Options: "i"}}},
			bson.M{"project_code": bson.M{"$regex": primitive.Regex{Pattern: regexEscape(q.Keyword), Options: "i"}}},
			bson.M{"owner_name": bson.M{"$regex": primitive.Regex{Pattern: regexEscape(q.Keyword), Options: "i"}}},
		}
	}
	total, err := r.coll().CountDocuments(ctx, filter)
	if err != nil {
		pmlog.Errorf("project List Count err=%v", err)
		return nil, 0, err
	}
	opt := options.Find().
		SetSkip((q.Page - 1) * q.Size).
		SetLimit(q.Size).
		SetSort(bson.D{{Key: "updated_at", Value: -1}})
	cur, err := r.coll().Find(ctx, filter, opt)
	if err != nil {
		pmlog.Errorf("project List Find err=%v", err)
		return nil, 0, err
	}
	defer cur.Close(ctx)
	out := make([]*Project, 0, q.Size)
	for cur.Next(ctx) {
		var p Project
		if err := cur.Decode(&p); err != nil {
			pmlog.Errorf("project List decode err=%v", err)
			return nil, 0, err
		}
		out = append(out, &p)
	}
	return out, total, nil
}

// UpdateStatus 仅当当前状态等于 fromStatus 时迁移；CAS 防并发。
func (r *MongoRepo) UpdateStatus(ctx context.Context, id, fromStatus, toStatus string, extra map[string]interface{}) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return errors.New("invalid id")
	}
	set := bson.M{"status": toStatus, "updated_at": time.Now()}
	for k, v := range extra {
		set[k] = v
	}
	res, err := r.coll().UpdateOne(ctx,
		bson.M{"_id": oid, "status": fromStatus},
		bson.M{"$set": set})
	if err != nil {
		pmlog.Errorf("project UpdateStatus err=%v id=%s", err, id)
		return err
	}
	if res.MatchedCount == 0 {
		return ErrStateConflict
	}
	return nil
}

// IncRevision 原子自增 current_revision，返回新值。
func (r *MongoRepo) IncRevision(ctx context.Context, id string) (int32, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return 0, errors.New("invalid id")
	}
	after := options.After
	opt := options.FindOneAndUpdate().SetReturnDocument(after)
	var updated Project
	err = r.coll().FindOneAndUpdate(ctx,
		bson.M{"_id": oid},
		bson.M{"$inc": bson.M{"current_revision": 1}, "$set": bson.M{"updated_at": time.Now()}},
		opt).Decode(&updated)
	if err != nil {
		pmlog.Errorf("project IncRevision err=%v id=%s", err, id)
		return 0, err
	}
	return updated.CurrentRevision, nil
}

// DecRevision 回滚 revision（保存无任何字段变更时调用）。
func (r *MongoRepo) DecRevision(ctx context.Context, id string) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return errors.New("invalid id")
	}
	if _, err := r.coll().UpdateOne(ctx,
		bson.M{"_id": oid},
		bson.M{"$inc": bson.M{"current_revision": -1}}); err != nil {
		pmlog.Errorf("project DecRevision err=%v id=%s", err, id)
		return err
	}
	return nil
}

// UpdateLastPhase 更新 last_phase（用于驳回返工标记）。
func (r *MongoRepo) UpdateLastPhase(ctx context.Context, id, phase string) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return errors.New("invalid id")
	}
	if _, err := r.coll().UpdateOne(ctx,
		bson.M{"_id": oid},
		bson.M{"$set": bson.M{"last_phase": phase, "updated_at": time.Now()}}); err != nil {
		pmlog.Errorf("project UpdateLastPhase err=%v id=%s", err, id)
		return err
	}
	return nil
}

// ExistsByNameAndOwnerActive 软重复检测：未归档且同名同业主存在。
func (r *MongoRepo) ExistsByNameAndOwnerActive(ctx context.Context, name, ownerName string) (bool, error) {
	count, err := r.coll().CountDocuments(ctx, bson.M{
		"project_name": name,
		"owner_name":   ownerName,
		"status":       bson.M{"$ne": StatusApproved},
	}, options.Count().SetLimit(1))
	if err != nil {
		pmlog.Errorf("project ExistsByNameAndOwnerActive err=%v", err)
		return false, err
	}
	return count > 0, nil
}

func regexEscape(s string) string {
	specials := `\.+*?()|[]{}^$`
	out := make([]rune, 0, len(s))
	for _, r := range s {
		for _, sp := range specials {
			if r == sp {
				out = append(out, '\\')
				break
			}
		}
		out = append(out, r)
	}
	return string(out)
}
