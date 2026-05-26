package project

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"projectManager/internal/modules/owner"
)

// memProjectRepo 内存版项目仓储。
type memProjectRepo struct {
	mu   sync.Mutex
	data map[string]*Project
}

func newMemProjectRepo() *memProjectRepo {
	return &memProjectRepo{data: map[string]*Project{}}
}

func (r *memProjectRepo) Insert(_ context.Context, p *Project) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if p.ID.IsZero() {
		p.ID = primitive.NewObjectID()
	}
	r.data[p.ID.Hex()] = p
	return nil
}
func (r *memProjectRepo) GetByID(_ context.Context, id string) (*Project, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.data[id]
	if !ok {
		return nil, nil
	}
	cp := *p
	return &cp, nil
}
func (r *memProjectRepo) List(_ context.Context, q ListQuery) ([]*Project, int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]*Project, 0)
	for _, p := range r.data {
		if q.Status != "" && p.Status != q.Status {
			continue
		}
		if q.ApplicantID != "" && p.ApplicantID != q.ApplicantID {
			continue
		}
		if q.Keyword != "" && !strings.Contains(p.ProjectName, q.Keyword) {
			continue
		}
		cp := *p
		out = append(out, &cp)
	}
	return out, int64(len(out)), nil
}
func (r *memProjectRepo) UpdateStatus(_ context.Context, id, from, to string, extra map[string]interface{}) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.data[id]
	if !ok {
		return ErrNotFound
	}
	if p.Status != from {
		return ErrStateConflict
	}
	p.Status = to
	_ = extra // 测试中不关心 extra 内容
	return nil
}
func (r *memProjectRepo) IncRevision(_ context.Context, id string) (int32, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.data[id]
	if !ok {
		return 0, ErrNotFound
	}
	p.CurrentRevision++
	return p.CurrentRevision, nil
}
func (r *memProjectRepo) DecRevision(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.data[id]
	if !ok {
		return ErrNotFound
	}
	p.CurrentRevision--
	return nil
}
func (r *memProjectRepo) UpdateLastPhase(_ context.Context, id, phase string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.data[id]
	if !ok {
		return ErrNotFound
	}
	p.LastPhase = phase
	return nil
}
func (r *memProjectRepo) ExistsByNameAndOwnerActive(_ context.Context, name, ownerName string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, p := range r.data {
		if p.ProjectName == name && p.OwnerName == ownerName && p.Status != StatusApproved {
			return true, nil
		}
	}
	return false, nil
}

// memOwnerRepo 内存版 owner.Repo。
type memOwnerRepo struct {
	mu   sync.Mutex
	data map[string]*owner.Owner
}

func newMemOwnerRepo() *memOwnerRepo { return &memOwnerRepo{data: map[string]*owner.Owner{}} }

func (r *memOwnerRepo) FindByName(_ context.Context, name string) (*owner.Owner, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	o, ok := r.data[name]
	if !ok {
		return nil, nil
	}
	cp := *o
	return &cp, nil
}
func (r *memOwnerRepo) UpsertByName(_ context.Context, o *owner.Owner) (*owner.Owner, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if o.Name == "" {
		return nil, errors.New("name empty")
	}
	if existed, ok := r.data[o.Name]; ok {
		cp := *existed
		return &cp, nil
	}
	o.ID = primitive.NewObjectID()
	r.data[o.Name] = o
	cp := *o
	return &cp, nil
}
func (r *memOwnerRepo) List(_ context.Context, _ string, _ int64) ([]*owner.Owner, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]*owner.Owner, 0, len(r.data))
	for _, v := range r.data {
		cp := *v
		out = append(out, &cp)
	}
	return out, nil
}

// 集成构造测试上下文。
func newProjectSvcForTest() (*Service, *memProjectRepo) {
	pRepo := newMemProjectRepo()
	oSvc := owner.NewService(newMemOwnerRepo())
	return NewService(pRepo, oSvc), pRepo
}

func TestCreate_MinimumInfoOK(t *testing.T) {
	svc, _ := newProjectSvcForTest()
	p, dup, err := svc.Create(context.Background(), &CreateInput{
		ProjectName: "示例项目A",
		OwnerName:   "示例业主单位",
		ApplicantID: "u1",
	})
	if err != nil {
		t.Fatalf("create err=%v", err)
	}
	if p.Status != StatusFormEditing {
		t.Fatalf("expect FORM_EDITING, got %s", p.Status)
	}
	if p.ProjectCode == "" {
		t.Fatalf("expect project_code generated")
	}
	if dup {
		t.Fatalf("first create should not be duplicate")
	}
}

func TestCreate_MissingFields(t *testing.T) {
	svc, _ := newProjectSvcForTest()
	cases := []*CreateInput{
		{ProjectName: "", OwnerName: "x", ApplicantID: "u1"},
		{ProjectName: "x", OwnerName: "", ApplicantID: "u1"},
		{ProjectName: "x", OwnerName: "y", ApplicantID: ""},
	}
	for i, c := range cases {
		if _, _, err := svc.Create(context.Background(), c); !errors.Is(err, ErrInvalidParam) {
			t.Fatalf("case %d expect ErrInvalidParam, got %v", i, err)
		}
	}
}

func TestCreate_DuplicateActive(t *testing.T) {
	svc, _ := newProjectSvcForTest()
	in := &CreateInput{ProjectName: "重复名", OwnerName: "甲方", ApplicantID: "u1"}
	if _, _, err := svc.Create(context.Background(), in); err != nil {
		t.Fatalf("first create err=%v", err)
	}
	_, dup, err := svc.Create(context.Background(), in)
	if err != nil {
		t.Fatalf("second create err=%v", err)
	}
	if !dup {
		t.Fatalf("expect duplicate flag true")
	}
}

func TestStateMachine_AllowAndDeny(t *testing.T) {
	cases := []struct {
		from, to string
		ok       bool
	}{
		{StatusFormEditing, StatusPendingReview, true},
		{StatusFormEditing, StatusApproved, false},
		{StatusPendingReview, StatusPendingApprove, true},
		{StatusPendingReview, StatusFormEditing, true},
		{StatusPendingApprove, StatusApproved, true},
		{StatusPendingApprove, StatusFormEditing, true},
		{StatusApproved, StatusFormEditing, false},
		{StatusDraft, StatusFormEditing, false}, // DRAFT 在本设计中不会出现
	}
	for _, c := range cases {
		if got := canTransit(c.from, c.to); got != c.ok {
			t.Fatalf("canTransit(%s,%s)=%v want %v", c.from, c.to, got, c.ok)
		}
	}
}
