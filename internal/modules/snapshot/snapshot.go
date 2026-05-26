// Package snapshot 提交定稿快照。
package snapshot

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

// Snapshot 表格快照。
type Snapshot struct {
	ID          primitive.ObjectID     `bson:"_id,omitempty" json:"id"`
	ProjectID   primitive.ObjectID     `bson:"project_id" json:"project_id"`
	Revision    int32                  `bson:"revision" json:"revision"`
	Content     map[string]interface{} `bson:"content" json:"content"`
	SubmittedBy string                 `bson:"submitted_by" json:"submitted_by"`
	SubmittedAt time.Time              `bson:"submitted_at" json:"submitted_at"`
}

// Repo 快照仓储。
type Repo interface {
	Insert(ctx context.Context, sn *Snapshot) (string, error)
	GetByID(ctx context.Context, id string) (*Snapshot, error)
	Latest(ctx context.Context, projectID string) (*Snapshot, error)
}

// MongoRepo Mongo 实现。
type MongoRepo struct{ store *mstore.Store }

// NewMongoRepo 构造。
func NewMongoRepo(s *mstore.Store) *MongoRepo { return &MongoRepo{store: s} }

func (r *MongoRepo) coll() *mongo.Collection { return r.store.Coll(mstore.CollProjectFormSnapshots) }

// Insert 插入。
func (r *MongoRepo) Insert(ctx context.Context, sn *Snapshot) (string, error) {
	if sn.ID.IsZero() {
		sn.ID = primitive.NewObjectID()
	}
	if _, err := r.coll().InsertOne(ctx, sn); err != nil {
		pmlog.Errorf("snapshot Insert err=%v", err)
		return "", err
	}
	return sn.ID.Hex(), nil
}

// GetByID 取快照。
func (r *MongoRepo) GetByID(ctx context.Context, id string) (*Snapshot, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, errors.New("invalid id")
	}
	var sn Snapshot
	err = r.coll().FindOne(ctx, bson.M{"_id": oid}).Decode(&sn)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		pmlog.Errorf("snapshot GetByID err=%v", err)
		return nil, err
	}
	return &sn, nil
}

// Latest 最近一份。
func (r *MongoRepo) Latest(ctx context.Context, projectID string) (*Snapshot, error) {
	pid, err := primitive.ObjectIDFromHex(projectID)
	if err != nil {
		return nil, errors.New("invalid project_id")
	}
	var sn Snapshot
	err = r.coll().FindOne(ctx,
		bson.M{"project_id": pid},
		options.FindOne().SetSort(bson.D{{Key: "revision", Value: -1}})).Decode(&sn)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		pmlog.Errorf("snapshot Latest err=%v", err)
		return nil, err
	}
	return &sn, nil
}

// Service 快照服务。
type Service struct{ repo Repo }

// NewService 构造。
func NewService(r Repo) *Service { return &Service{repo: r} }

// Create 新建快照。
func (s *Service) Create(ctx context.Context, sn *Snapshot) (string, error) {
	return s.repo.Insert(ctx, sn)
}

// GetByID 取快照。
func (s *Service) GetByID(ctx context.Context, id string) (*Snapshot, error) {
	return s.repo.GetByID(ctx, id)
}

// Latest 最近快照。
func (s *Service) Latest(ctx context.Context, projectID string) (*Snapshot, error) {
	return s.repo.Latest(ctx, projectID)
}
