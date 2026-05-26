package project

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"strings"
	"time"

	pmlog "projectManager/internal/log"
	"projectManager/internal/modules/owner"
)

// 错误。
var (
	ErrStateConflict = errors.New("state_conflict")
	ErrNotFound      = errors.New("not_found")
	ErrInvalidParam  = errors.New("invalid_param")
)

// CreateInput 立项入参。
type CreateInput struct {
	ProjectName  string
	OwnerName    string
	OwnerType    string
	ContactName  string
	ContactPhone string
	ContactEmail string
	Address      string

	ApplicantID   string
	ApplicantName string
}

// Service 项目服务。
type Service struct {
	repo     Repo
	ownerSvc *owner.Service
}

// NewService 构造。
func NewService(repo Repo, ownerSvc *owner.Service) *Service {
	return &Service{repo: repo, ownerSvc: ownerSvc}
}

// Create 立项（最小信息）。
func (s *Service) Create(ctx context.Context, in *CreateInput) (*Project, bool, error) {
	in.ProjectName = strings.TrimSpace(in.ProjectName)
	in.OwnerName = strings.TrimSpace(in.OwnerName)
	if in.ProjectName == "" {
		return nil, false, fmt.Errorf("%w: project_name required", ErrInvalidParam)
	}
	if len([]rune(in.ProjectName)) > 100 {
		return nil, false, fmt.Errorf("%w: project_name too long", ErrInvalidParam)
	}
	if in.OwnerName == "" {
		return nil, false, fmt.Errorf("%w: owner_name required", ErrInvalidParam)
	}
	if in.ApplicantID == "" {
		return nil, false, fmt.Errorf("%w: applicant required", ErrInvalidParam)
	}

	o, err := s.ownerSvc.EnsureByName(ctx, &owner.Owner{
		Name:         in.OwnerName,
		OwnerType:    in.OwnerType,
		ContactName:  in.ContactName,
		ContactPhone: in.ContactPhone,
		ContactEmail: in.ContactEmail,
		Address:      in.Address,
	})
	if err != nil {
		pmlog.Errorf("project.Create owner err=%v", err)
		return nil, false, err
	}

	dup, err := s.repo.ExistsByNameAndOwnerActive(ctx, in.ProjectName, in.OwnerName)
	if err != nil {
		return nil, false, err
	}

	p := &Project{
		ProjectCode:     genProjectCode(),
		ProjectName:     in.ProjectName,
		OwnerID:         o.ID,
		OwnerName:       o.Name,
		Status:          StatusFormEditing,
		CurrentRevision: 0,
		ApplicantID:     in.ApplicantID,
		ApplicantName:   in.ApplicantName,
		LastPhase:       PhaseFormEditing,
	}
	if err := s.repo.Insert(ctx, p); err != nil {
		return nil, false, err
	}
	pmlog.Infof("project created project_id=%s code=%s applicant=%s", p.ID.Hex(), p.ProjectCode, in.ApplicantID)
	return p, dup, nil
}

// GetByID 详情。
func (s *Service) GetByID(ctx context.Context, id string) (*Project, error) {
	p, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, ErrNotFound
	}
	return p, nil
}

// List 列表。
func (s *Service) List(ctx context.Context, q ListQuery) ([]*Project, int64, error) {
	return s.repo.List(ctx, q)
}

// IncRevision 增加版本号（供 form-value 模块调用）。
func (s *Service) IncRevision(ctx context.Context, id string) (int32, error) {
	return s.repo.IncRevision(ctx, id)
}

// DecRevision 回滚版本号。
func (s *Service) DecRevision(ctx context.Context, id string) error {
	return s.repo.DecRevision(ctx, id)
}

// TransitStatus 状态机白名单迁移。
func (s *Service) TransitStatus(ctx context.Context, id, from, to string, extra map[string]interface{}) error {
	if !canTransit(from, to) {
		return ErrStateConflict
	}
	return s.repo.UpdateStatus(ctx, id, from, to, extra)
}

// UpdateLastPhase 更新阶段标识。
func (s *Service) UpdateLastPhase(ctx context.Context, id, phase string) error {
	return s.repo.UpdateLastPhase(ctx, id, phase)
}

// canTransit 状态机矩阵。
func canTransit(from, to string) bool {
	allow := map[string]map[string]bool{
		StatusFormEditing: {
			StatusPendingReview: true,
		},
		StatusPendingReview: {
			StatusPendingApprove: true,
			StatusFormEditing:    true, // reject 回填
		},
		StatusPendingApprove: {
			StatusApproved:    true,
			StatusFormEditing: true, // reject 回填
		},
	}
	to2, ok := allow[from]
	if !ok {
		return false
	}
	return to2[to]
}

// genProjectCode 生成项目编号：PJ + yyyyMMdd + 6位随机十六进制。
func genProjectCode() string {
	now := time.Now().Format("20060102")
	buf := make([]byte, 3)
	_, _ = rand.Read(buf)
	return fmt.Sprintf("PJ%s%X", now, buf)
}
