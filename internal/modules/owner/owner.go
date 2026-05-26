// Package owner 业主单位领域模型与服务。
package owner

import (
	"context"
	"errors"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	pmlog "projectManager/internal/log"
	mstore "projectManager/internal/store/mongo"
)

// Owner 业主单位。
type Owner struct {
	ID           primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Name         string             `bson:"name" json:"name"`
	OwnerType    string             `bson:"owner_type" json:"owner_type"`
	ContactName  string             `bson:"contact_name" json:"contact_name"`
	ContactPhone string             `bson:"contact_phone" json:"contact_phone"`
	ContactEmail string             `bson:"contact_email" json:"contact_email"`
	Address      string             `bson:"address" json:"address"`
	CreatedAt    time.Time          `bson:"created_at" json:"created_at"`
}

// Repo 业主单位存储接口。便于单测替换。
type Repo interface {
	FindByName(ctx context.Context, name string) (*Owner, error)
	UpsertByName(ctx context.Context, o *Owner) (*Owner, error)
	List(ctx context.Context, keyword string, limit int64) ([]*Owner, error)
}

// MongoRepo 基于 MongoDB 的实现。
type MongoRepo struct{ store *mstore.Store }

// NewMongoRepo 构造。
func NewMongoRepo(s *mstore.Store) *MongoRepo { return &MongoRepo{store: s} }

func (r *MongoRepo) coll() *mongo.Collection {
	return r.store.Coll(mstore.CollOwners)
}

// FindByName 按名称精确匹配。
func (r *MongoRepo) FindByName(ctx context.Context, name string) (*Owner, error) {
	var o Owner
	err := r.coll().FindOne(ctx, bson.M{"name": name}).Decode(&o)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		pmlog.Errorf("owner FindByName err=%v name=%s", err, name)
		return nil, err
	}
	return &o, nil
}

// UpsertByName 按 name upsert。
func (r *MongoRepo) UpsertByName(ctx context.Context, o *Owner) (*Owner, error) {
	if strings.TrimSpace(o.Name) == "" {
		return nil, errors.New("owner.name empty")
	}
	now := time.Now()
	filter := bson.M{"name": o.Name}
	update := bson.M{
		"$setOnInsert": bson.M{
			"name":       o.Name,
			"created_at": now,
		},
		"$set": bson.M{
			"owner_type":    o.OwnerType,
			"contact_name":  o.ContactName,
			"contact_phone": o.ContactPhone,
			"contact_email": o.ContactEmail,
			"address":       o.Address,
		},
	}
	opt := options.Update().SetUpsert(true)
	if _, err := r.coll().UpdateOne(ctx, filter, update, opt); err != nil {
		pmlog.Errorf("owner Upsert err=%v name=%s", err, o.Name)
		return nil, err
	}
	return r.FindByName(ctx, o.Name)
}

// List 分页列表（简易：limit + 关键字过滤）。
func (r *MongoRepo) List(ctx context.Context, keyword string, limit int64) ([]*Owner, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	filter := bson.M{}
	if keyword != "" {
		filter["name"] = bson.M{"$regex": primitive.Regex{Pattern: regexEscape(keyword), Options: "i"}}
	}
	opt := options.Find().SetLimit(limit).SetSort(bson.D{{Key: "created_at", Value: -1}})
	cur, err := r.coll().Find(ctx, filter, opt)
	if err != nil {
		pmlog.Errorf("owner List err=%v", err)
		return nil, err
	}
	defer cur.Close(ctx)
	out := make([]*Owner, 0, limit)
	for cur.Next(ctx) {
		var o Owner
		if err := cur.Decode(&o); err != nil {
			pmlog.Errorf("owner List decode err=%v", err)
			return nil, err
		}
		out = append(out, &o)
	}
	return out, nil
}

// regexEscape 简单转义，避免用户输入构造异常正则。
func regexEscape(s string) string {
	specials := `\.+*?()|[]{}^$`
	var b strings.Builder
	for _, r := range s {
		if strings.ContainsRune(specials, r) {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// Service 业务封装。
type Service struct{ repo Repo }

// NewService 构造。
func NewService(r Repo) *Service { return &Service{repo: r} }

// EnsureByName 业主单位若存在则复用，否则创建。
func (s *Service) EnsureByName(ctx context.Context, in *Owner) (*Owner, error) {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return nil, errors.New("owner_name required")
	}
	existing, err := s.repo.FindByName(ctx, in.Name)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}
	return s.repo.UpsertByName(ctx, in)
}

// List 列表。
func (s *Service) List(ctx context.Context, keyword string, limit int64) ([]*Owner, error) {
	return s.repo.List(ctx, keyword, limit)
}
