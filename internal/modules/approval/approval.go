// Package approval 审核 / 审批 / 提交定稿。
package approval

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
	"projectManager/internal/modules/formtemplate"
	"projectManager/internal/modules/formvalue"
	"projectManager/internal/modules/project"
	"projectManager/internal/modules/snapshot"
	mstore "projectManager/internal/store/mongo"
)

// 错误。
var (
	ErrStateConflict = errors.New("state_conflict")
	ErrCommentEmpty  = errors.New("comment_required_on_reject")
	ErrValidation    = errors.New("validation_failed")
	ErrInvalidParam  = errors.New("invalid_param")
)

// Action 类型。
const (
	ActionSubmit  = "submit"
	ActionApprove = "approve"
	ActionReject  = "reject"
)

// Level 级别。
const (
	LevelReview = 1
	LevelFinal  = 2
)

// Event 审批事件。
type Event struct {
	ID           primitive.ObjectID  `bson:"_id,omitempty" json:"id"`
	ProjectID    primitive.ObjectID  `bson:"project_id" json:"project_id"`
	Level        int32               `bson:"level" json:"level"`
	Action       string              `bson:"action" json:"action"`
	OperatorID   string              `bson:"operator_id" json:"operator_id"`
	OperatorName string              `bson:"operator_name" json:"operator_name"`
	OperatorRole string              `bson:"operator_role" json:"operator_role"`
	Comment      string              `bson:"comment" json:"comment"`
	SnapshotID   *primitive.ObjectID `bson:"snapshot_id,omitempty" json:"snapshot_id,omitempty"`
	CreatedAt    time.Time           `bson:"created_at" json:"created_at"`
}

// Repo 事件仓储。
type Repo interface {
	Insert(ctx context.Context, e *Event) error
	List(ctx context.Context, projectID string) ([]*Event, error)
}

// MongoRepo Mongo 实现。
type MongoRepo struct{ store *mstore.Store }

// NewMongoRepo 构造。
func NewMongoRepo(s *mstore.Store) *MongoRepo { return &MongoRepo{store: s} }

func (r *MongoRepo) coll() *mongo.Collection { return r.store.Coll(mstore.CollApprovalEvents) }

// Insert 插入。
func (r *MongoRepo) Insert(ctx context.Context, e *Event) error {
	if e.ID.IsZero() {
		e.ID = primitive.NewObjectID()
	}
	if _, err := r.coll().InsertOne(ctx, e); err != nil {
		pmlog.Errorf("approval Insert err=%v", err)
		return err
	}
	return nil
}

// List 查询某项目所有审批事件。
func (r *MongoRepo) List(ctx context.Context, projectID string) ([]*Event, error) {
	pid, err := primitive.ObjectIDFromHex(projectID)
	if err != nil {
		return nil, errors.New("invalid project_id")
	}
	cur, err := r.coll().Find(ctx,
		bson.M{"project_id": pid},
		options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}}))
	if err != nil {
		pmlog.Errorf("approval List err=%v", err)
		return nil, err
	}
	defer cur.Close(ctx)
	out := make([]*Event, 0)
	for cur.Next(ctx) {
		var e Event
		if err := cur.Decode(&e); err != nil {
			pmlog.Errorf("approval List decode err=%v", err)
			return nil, err
		}
		out = append(out, &e)
	}
	return out, nil
}

// SubmitResult 提交定稿返回。
type SubmitResult struct {
	Status        string   `json:"status"`
	SnapshotID    string   `json:"snapshot_id"`
	MissingFields []string `json:"missing_fields,omitempty"`
}

// Service 审批服务。
type Service struct {
	repo        Repo
	projectSvc  *project.Service
	tmplSvc     *formtemplate.Service
	formValSvc  *formvalue.Service
	snapshotSvc *snapshot.Service
}

// NewService 构造。
func NewService(repo Repo, projectSvc *project.Service, tmplSvc *formtemplate.Service,
	fv *formvalue.Service, snSvc *snapshot.Service) *Service {
	return &Service{repo: repo, projectSvc: projectSvc, tmplSvc: tmplSvc, formValSvc: fv, snapshotSvc: snSvc}
}

// Submit 申请人提交定稿。
func (s *Service) Submit(ctx context.Context, projectID string, opID, opName, opRole string) (*SubmitResult, error) {
	p, err := s.projectSvc.GetByID(ctx, projectID)
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
	values, err := s.formValSvc.CurrentValues(ctx, projectID)
	if err != nil {
		return nil, err
	}

	missing := validateRequired(tmpl, values)
	if len(missing) > 0 {
		return &SubmitResult{Status: p.Status, MissingFields: missing}, ErrValidation
	}

	// 写快照。
	sn := &snapshot.Snapshot{
		ProjectID:   p.ID,
		Revision:    p.CurrentRevision,
		Content:     values,
		SubmittedBy: opID,
		SubmittedAt: time.Now(),
	}
	snID, err := s.snapshotSvc.Create(ctx, sn)
	if err != nil {
		return nil, err
	}

	// 状态迁移。
	if err := s.projectSvc.TransitStatus(ctx, projectID,
		project.StatusFormEditing, project.StatusPendingReview, nil); err != nil {
		pmlog.Errorf("approval Submit transit err=%v project_id=%s", err, projectID)
		return nil, err
	}

	// 写事件。
	snOID, _ := primitive.ObjectIDFromHex(snID)
	if err := s.repo.Insert(ctx, &Event{
		ProjectID:    p.ID,
		Level:        LevelReview,
		Action:       ActionSubmit,
		OperatorID:   opID,
		OperatorName: opName,
		OperatorRole: opRole,
		SnapshotID:   &snOID,
		CreatedAt:    time.Now(),
	}); err != nil {
		return nil, err
	}

	pmlog.Infof("approval Submit ok project_id=%s revision=%d snapshot=%s operator=%s", projectID, p.CurrentRevision, snID, opID)
	return &SubmitResult{Status: project.StatusPendingReview, SnapshotID: snID}, nil
}

// Decide 一级审核 / 二级审批的统一处理。
// level: 1=review, 2=final
// action: approve / reject
func (s *Service) Decide(ctx context.Context, projectID string, level int32, action, comment, opID, opName, opRole string) (string, error) {
	action = strings.ToLower(strings.TrimSpace(action))
	if action != ActionApprove && action != ActionReject {
		return "", ErrInvalidParam
	}
	if action == ActionReject && strings.TrimSpace(comment) == "" {
		return "", ErrCommentEmpty
	}

	p, err := s.projectSvc.GetByID(ctx, projectID)
	if err != nil {
		return "", err
	}

	var fromStatus, toStatus string
	switch level {
	case LevelReview:
		fromStatus = project.StatusPendingReview
		if action == ActionApprove {
			toStatus = project.StatusPendingApprove
		} else {
			toStatus = project.StatusFormEditing
		}
	case LevelFinal:
		fromStatus = project.StatusPendingApprove
		if action == ActionApprove {
			toStatus = project.StatusApproved
		} else {
			toStatus = project.StatusFormEditing
		}
	default:
		return "", ErrInvalidParam
	}
	if p.Status != fromStatus {
		return "", ErrStateConflict
	}

	extra := map[string]interface{}{}
	if action == ActionApprove && level == LevelFinal {
		now := time.Now()
		extra["approved_at"] = now
	}

	if err := s.projectSvc.TransitStatus(ctx, projectID, fromStatus, toStatus, extra); err != nil {
		return "", err
	}

	// 驳回则更新阶段标识，便于后续 ChangeLog phase=REJECTED_REWORK。
	if action == ActionReject {
		if err := s.projectSvc.UpdateLastPhase(ctx, projectID, project.PhaseRejectedRework); err != nil {
			pmlog.Errorf("approval UpdateLastPhase err=%v", err)
			// 不阻断主流程
		}
	} else if action == ActionApprove && level == LevelFinal {
		_ = s.projectSvc.UpdateLastPhase(ctx, projectID, project.PhaseFormEditing)
	}

	if err := s.repo.Insert(ctx, &Event{
		ProjectID:    p.ID,
		Level:        level,
		Action:       action,
		OperatorID:   opID,
		OperatorName: opName,
		OperatorRole: opRole,
		Comment:      comment,
		CreatedAt:    time.Now(),
	}); err != nil {
		return "", err
	}

	pmlog.Infof("approval Decide ok project_id=%s level=%d action=%s to=%s operator=%s", projectID, level, action, toStatus, opID)
	return toStatus, nil
}

// List 审批事件。
func (s *Service) List(ctx context.Context, projectID string) ([]*Event, error) {
	return s.repo.List(ctx, projectID)
}

// validateRequired 校验所有 required 字段非空。
func validateRequired(tmpl *formtemplate.Template, values map[string]interface{}) []string {
	if tmpl == nil {
		return nil
	}
	var missing []string
	for _, sec := range tmpl.Sections {
		for _, f := range sec.Fields {
			if !f.Required {
				continue
			}
			v, ok := values[f.FieldKey]
			if !ok || isEmpty(v) {
				missing = append(missing, f.FieldKey)
			}
		}
	}
	return missing
}

func isEmpty(v interface{}) bool {
	if v == nil {
		return true
	}
	switch x := v.(type) {
	case string:
		return strings.TrimSpace(x) == ""
	case []interface{}:
		return len(x) == 0
	case map[string]interface{}:
		return len(x) == 0
	}
	return false
}
