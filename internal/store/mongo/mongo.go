// Package mongo 封装 MongoDB 客户端与集合常量。
// 规则 R05：仅暴露通过 BSON 参数化方式读写。
package mongo

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// 集合名常量，避免硬编码字符串散落各处。
const (
	CollProjects             = "projects"
	CollOwners               = "owners"
	CollFormTemplates        = "form_templates"
	CollProjectFormValues    = "project_form_values"
	CollProjectChangeLogs    = "project_change_logs"
	CollProjectFormSnapshots = "project_form_snapshots"
	CollApprovalEvents       = "approval_events"
)

// Store 持有 client 与 db 句柄。
type Store struct {
	Client *mongo.Client
	DB     *mongo.Database
}

// Connect 创建并 ping。
func Connect(ctx context.Context, uri, db string) (*Store, error) {
	c, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return nil, err
	}
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := c.Ping(pingCtx, nil); err != nil {
		return nil, err
	}
	return &Store{Client: c, DB: c.Database(db)}, nil
}

// Close 释放连接。
func (s *Store) Close(ctx context.Context) error {
	if s.Client == nil {
		return nil
	}
	return s.Client.Disconnect(ctx)
}

// Coll 取集合句柄。
func (s *Store) Coll(name string) *mongo.Collection {
	return s.DB.Collection(name)
}
