// Package formvalue 项目表格值读写 + ChangeLog 编排。
package formvalue

import (
	"context"
	"errors"
	"reflect"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	pmlog "projectManager/internal/log"
	"projectManager/internal/modules/changelog"
	"projectManager/internal/modules/formtemplate"
	"projectManager/internal/modules/project"
	mstore "projectManager/internal/store/mongo"
)

// 错误。
var (
	ErrStateConflict = errors.New("state_conflict")
	ErrFieldUnknown  = errors.New("field_unknown")
	ErrFieldForbid   = errors.New("field_not_editable_for_role")
)

// FieldValue 字段当前值。
type FieldValue struct {
	ProjectID primitive.ObjectID `bson:"project_id" json:"project_id"`
	FieldKey  string             `bson:"field_key" json:"field_key"`
	Value     interface{}        `bson:"value" json:"value"`
	UpdatedBy string             `bson:"updated_by" json:"updated_by"`
	UpdatedAt time.Time          `bson:"updated_at" json:"updated_at"`
	Revision  int32              `bson:"revision" json:"revision"`
}

// Repo 表格值仓储。
type Repo interface {
	GetAll(ctx context.Context, projectID string) ([]*FieldValue, error)
	Upsert(ctx context.Context, fv *FieldValue) error
}

// MongoRepo Mongo 实现。
type MongoRepo struct{ store *mstore.Store }

// NewMongoRepo 构造。
func NewMongoRepo(s *mstore.Store) *MongoRepo { return &MongoRepo{store: s} }

func (r *MongoRepo) coll() *mongo.Collection { return r.store.Coll(mstore.CollProjectFormValues) }

// GetAll 取项目所有字段值。
func (r *MongoRepo) GetAll(ctx context.Context, projectID string) ([]*FieldValue, error) {
	pid, err := primitive.ObjectIDFromHex(projectID)
	if err != nil {
		return nil, errors.New("invalid project_id")
	}
	cur, err := r.coll().Find(ctx, bson.M{"project_id": pid})
	if err != nil {
		pmlog.Errorf("formvalue GetAll err=%v", err)
		return nil, err
	}
	defer cur.Close(ctx)
	out := make([]*FieldValue, 0)
	for cur.Next(ctx) {
		var v FieldValue
		if err := cur.Decode(&v); err != nil {
			pmlog.Errorf("formvalue GetAll decode err=%v", err)
			return nil, err
		}
		out = append(out, &v)
	}
	return out, nil
}

// Upsert 写入字段值。
func (r *MongoRepo) Upsert(ctx context.Context, fv *FieldValue) error {
	filter := bson.M{"project_id": fv.ProjectID, "field_key": fv.FieldKey}
	update := bson.M{"$set": bson.M{
		"value":      fv.Value,
		"updated_by": fv.UpdatedBy,
		"updated_at": fv.UpdatedAt,
		"revision":   fv.Revision,
	}}
	if _, err := r.coll().UpdateOne(ctx, filter, update, options.Update().SetUpsert(true)); err != nil {
		pmlog.Errorf("formvalue Upsert err=%v field=%s", err, fv.FieldKey)
		return err
	}
	return nil
}

// Change 一次保存的单个字段变更入参。
type Change struct {
	FieldKey string      `json:"field_key"`
	NewValue interface{} `json:"new_value"`
	Remark   string      `json:"remark"`
}

// SaveInput 表格保存请求。
type SaveInput struct {
	ProjectID    string
	OperatorID   string
	OperatorName string
	OperatorRole string
	Changes      []Change
}

// SaveResult 保存结果。
type SaveResult struct {
	Revision     int32 `json:"revision"`
	AppliedCount int   `json:"applied_count"`
	SkippedCount int   `json:"skipped_count"`
}

// Service 编排表格值与 ChangeLog。
type Service struct {
	repo       Repo
	tmplSvc    *formtemplate.Service
	projectSvc *project.Service
	logSvc     *changelog.Service
}

// NewService 构造。
func NewService(repo Repo, tmplSvc *formtemplate.Service, projectSvc *project.Service, logSvc *changelog.Service) *Service {
	return &Service{repo: repo, tmplSvc: tmplSvc, projectSvc: projectSvc, logSvc: logSvc}
}

// GetForm 取项目当前表格（模板 + 当前值）。
func (s *Service) GetForm(ctx context.Context, projectID string) (*project.Project, *formtemplate.Template, map[string]interface{}, error) {
	p, err := s.projectSvc.GetByID(ctx, projectID)
	if err != nil {
		return nil, nil, nil, err
	}
	tmpl, err := s.tmplSvc.GetActive(ctx)
	if err != nil {
		return nil, nil, nil, err
	}
	values, err := s.repo.GetAll(ctx, projectID)
	if err != nil {
		return nil, nil, nil, err
	}
	vmap := make(map[string]interface{}, len(values))
	for _, v := range values {
		vmap[v.FieldKey] = v.Value
	}
	return p, tmpl, vmap, nil
}

// CurrentValues 仅取当前值字典（供 snapshot/校验复用）。
func (s *Service) CurrentValues(ctx context.Context, projectID string) (map[string]interface{}, error) {
	values, err := s.repo.GetAll(ctx, projectID)
	if err != nil {
		return nil, err
	}
	out := make(map[string]interface{}, len(values))
	for _, v := range values {
		out[v.FieldKey] = v.Value
	}
	return out, nil
}

// Save 保存字段：状态校验、字段权限校验、ChangeLog 生成。
func (s *Service) Save(ctx context.Context, in *SaveInput) (*SaveResult, error) {
	if in == nil || in.ProjectID == "" || in.OperatorID == "" {
		return nil, errors.New("invalid_param")
	}
	if len(in.Changes) == 0 {
		return nil, errors.New("invalid_param: empty changes")
	}

	p, err := s.projectSvc.GetByID(ctx, in.ProjectID)
	if err != nil {
		return nil, err
	}
	if p.Status != project.StatusFormEditing {
		return nil, ErrStateConflict
	}

	tmpl, err := s.tmplSvc.GetActive(ctx)
	if err != nil {
		return nil, err
	}
	fieldMap := formtemplate.FieldMap(tmpl)

	// 校验字段是否存在，以及当前角色是否可编辑。
	for _, c := range in.Changes {
		f, ok := fieldMap[c.FieldKey]
		if !ok {
			return nil, ErrFieldUnknown
		}
		if !roleCan(f.EditableRoles, in.OperatorRole) {
			return nil, ErrFieldForbid
		}
	}

	// 取当前值。
	cur, err := s.CurrentValues(ctx, in.ProjectID)
	if err != nil {
		return nil, err
	}

	// 预筛实际有变化的字段。
	type pending struct {
		change Change
		oldVal interface{}
		field  formtemplate.Field
	}
	var actual []pending
	for _, c := range in.Changes {
		old := cur[c.FieldKey]
		if equalValue(old, c.NewValue) {
			continue
		}
		actual = append(actual, pending{change: c, oldVal: old, field: fieldMap[c.FieldKey]})
	}

	if len(actual) == 0 {
		pmlog.Infof("formvalue Save no-op project_id=%s operator=%s", in.ProjectID, in.OperatorID)
		return &SaveResult{Revision: p.CurrentRevision, AppliedCount: 0, SkippedCount: len(in.Changes)}, nil
	}

	// 分配 revision。
	rev, err := s.projectSvc.IncRevision(ctx, in.ProjectID)
	if err != nil {
		return nil, err
	}

	phase := project.PhaseFormEditing
	if p.LastPhase == project.PhaseRejectedRework {
		phase = project.PhaseRejectedRework
	}

	now := time.Now()
	logs := make([]*changelog.ChangeLog, 0, len(actual))
	for _, a := range actual {
		// 写入字段值。
		fv := &FieldValue{
			ProjectID: p.ID,
			FieldKey:  a.change.FieldKey,
			Value:     a.change.NewValue,
			UpdatedBy: in.OperatorID,
			UpdatedAt: now,
			Revision:  rev,
		}
		if err := s.repo.Upsert(ctx, fv); err != nil {
			// 写值失败：回滚 revision，避免空洞。
			_ = s.projectSvc.DecRevision(ctx, in.ProjectID)
			return nil, err
		}
		logs = append(logs, &changelog.ChangeLog{
			ProjectID:    p.ID,
			FieldKey:     a.change.FieldKey,
			FieldLabel:   a.field.Label,
			OldValue:     a.oldVal,
			NewValue:     a.change.NewValue,
			OperatorID:   in.OperatorID,
			OperatorName: in.OperatorName,
			OperatorRole: in.OperatorRole,
			OperatedAt:   now,
			Revision:     rev,
			Remark:       a.change.Remark,
			Phase:        phase,
			Hidden:       false,
		})
	}
	if err := s.logSvc.InsertMany(ctx, logs); err != nil {
		// 日志写失败不再回滚字段值（值与日志可能短暂不一致），但记录 ERROR；上层可重试。
		pmlog.Errorf("formvalue Save changelog insert err=%v project_id=%s rev=%d", err, in.ProjectID, rev)
		return nil, err
	}
	pmlog.Infof("formvalue Save ok project_id=%s rev=%d applied=%d operator=%s role=%s",
		in.ProjectID, rev, len(actual), in.OperatorID, in.OperatorRole)
	return &SaveResult{Revision: rev, AppliedCount: len(actual), SkippedCount: len(in.Changes) - len(actual)}, nil
}

// roleCan 判定角色是否可编辑。admin 始终允许。
func roleCan(allowed []string, role string) bool {
	if role == "admin" {
		return true
	}
	for _, r := range allowed {
		if r == role {
			return true
		}
	}
	return false
}

// equalValue 用于判定字段值是否未变化。reflect.DeepEqual 可比较 nil/基本类型/嵌套 map/slice。
// 同时把 nil 与空字符串视为不同（保留首次填写记录）。
func equalValue(a, b interface{}) bool {
	return reflect.DeepEqual(a, b)
}
