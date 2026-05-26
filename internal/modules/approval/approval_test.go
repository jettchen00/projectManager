package approval

import (
	"context"
	"errors"
	"sync"
	"testing"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"projectManager/internal/modules/changelog"
	"projectManager/internal/modules/formtemplate"
	"projectManager/internal/modules/formvalue"
	"projectManager/internal/modules/owner"
	"projectManager/internal/modules/project"
	"projectManager/internal/modules/snapshot"
)

// ---- in-memory repos ----

type memProjectRepo struct {
	mu   sync.Mutex
	data map[string]*project.Project
}

func newMemProjectRepo() *memProjectRepo {
	return &memProjectRepo{data: map[string]*project.Project{}}
}
func (r *memProjectRepo) Insert(_ context.Context, p *project.Project) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if p.ID.IsZero() {
		p.ID = primitive.NewObjectID()
	}
	r.data[p.ID.Hex()] = p
	return nil
}
func (r *memProjectRepo) GetByID(_ context.Context, id string) (*project.Project, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.data[id]
	if !ok {
		return nil, nil
	}
	cp := *p
	return &cp, nil
}
func (r *memProjectRepo) List(_ context.Context, _ project.ListQuery) ([]*project.Project, int64, error) {
	return nil, 0, nil
}
func (r *memProjectRepo) UpdateStatus(_ context.Context, id, from, to string, _ map[string]interface{}) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.data[id]
	if !ok {
		return project.ErrNotFound
	}
	if p.Status != from {
		return project.ErrStateConflict
	}
	p.Status = to
	return nil
}
func (r *memProjectRepo) IncRevision(_ context.Context, id string) (int32, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.data[id]
	if !ok {
		return 0, project.ErrNotFound
	}
	p.CurrentRevision++
	return p.CurrentRevision, nil
}
func (r *memProjectRepo) DecRevision(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.data[id]
	if !ok {
		return project.ErrNotFound
	}
	p.CurrentRevision--
	return nil
}
func (r *memProjectRepo) UpdateLastPhase(_ context.Context, id, phase string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.data[id]
	if !ok {
		return project.ErrNotFound
	}
	p.LastPhase = phase
	return nil
}
func (r *memProjectRepo) ExistsByNameAndOwnerActive(_ context.Context, _, _ string) (bool, error) {
	return false, nil
}

type memOwnerRepo struct{ data map[string]*owner.Owner }

func newMemOwnerRepo() *memOwnerRepo { return &memOwnerRepo{data: map[string]*owner.Owner{}} }
func (r *memOwnerRepo) FindByName(_ context.Context, name string) (*owner.Owner, error) {
	o, ok := r.data[name]
	if !ok {
		return nil, nil
	}
	cp := *o
	return &cp, nil
}
func (r *memOwnerRepo) UpsertByName(_ context.Context, o *owner.Owner) (*owner.Owner, error) {
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
	return nil, nil
}

type memTmplRepo struct{ t *formtemplate.Template }

func (r *memTmplRepo) GetActive(_ context.Context) (*formtemplate.Template, error) {
	return r.t, nil
}
func (r *memTmplRepo) InsertIfEmpty(_ context.Context, t *formtemplate.Template) error {
	if r.t == nil {
		r.t = t
	}
	return nil
}

type memChangeLogRepo struct {
	mu   sync.Mutex
	data []*changelog.ChangeLog
}

func newMemLogRepo() *memChangeLogRepo { return &memChangeLogRepo{} }
func (r *memChangeLogRepo) InsertMany(_ context.Context, items []*changelog.ChangeLog) error {
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
func (r *memChangeLogRepo) List(_ context.Context, _ changelog.Query) ([]*changelog.ChangeLog, int64, error) {
	return nil, 0, nil
}
func (r *memChangeLogRepo) ListByField(_ context.Context, _, _ string) ([]*changelog.ChangeLog, error) {
	return nil, nil
}
func (r *memChangeLogRepo) ListByRevisionRange(_ context.Context, _ string, _, _ int32) ([]*changelog.ChangeLog, error) {
	return nil, nil
}

type memValueRepo struct {
	mu   sync.Mutex
	data map[string]*formvalue.FieldValue
}

func newMemValueRepo() *memValueRepo {
	return &memValueRepo{data: map[string]*formvalue.FieldValue{}}
}
func (r *memValueRepo) GetAll(_ context.Context, projectID string) ([]*formvalue.FieldValue, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	pid, _ := primitive.ObjectIDFromHex(projectID)
	out := make([]*formvalue.FieldValue, 0)
	for _, v := range r.data {
		if v.ProjectID == pid {
			cp := *v
			out = append(out, &cp)
		}
	}
	return out, nil
}
func (r *memValueRepo) Upsert(_ context.Context, fv *formvalue.FieldValue) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.data[fv.ProjectID.Hex()+"|"+fv.FieldKey] = fv
	return nil
}

type memSnapshotRepo struct {
	mu   sync.Mutex
	data map[string]*snapshot.Snapshot
}

func newMemSnRepo() *memSnapshotRepo { return &memSnapshotRepo{data: map[string]*snapshot.Snapshot{}} }
func (r *memSnapshotRepo) Insert(_ context.Context, sn *snapshot.Snapshot) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if sn.ID.IsZero() {
		sn.ID = primitive.NewObjectID()
	}
	r.data[sn.ID.Hex()] = sn
	return sn.ID.Hex(), nil
}
func (r *memSnapshotRepo) GetByID(_ context.Context, id string) (*snapshot.Snapshot, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	sn, ok := r.data[id]
	if !ok {
		return nil, nil
	}
	cp := *sn
	return &cp, nil
}
func (r *memSnapshotRepo) Latest(_ context.Context, projectID string) (*snapshot.Snapshot, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	pid, _ := primitive.ObjectIDFromHex(projectID)
	var latest *snapshot.Snapshot
	for _, sn := range r.data {
		if sn.ProjectID != pid {
			continue
		}
		if latest == nil || sn.Revision > latest.Revision {
			latest = sn
		}
	}
	if latest == nil {
		return nil, nil
	}
	cp := *latest
	return &cp, nil
}

type memApprovalRepo struct {
	mu   sync.Mutex
	data []*Event
}

func newMemApprovalRepo() *memApprovalRepo { return &memApprovalRepo{} }
func (r *memApprovalRepo) Insert(_ context.Context, e *Event) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if e.ID.IsZero() {
		e.ID = primitive.NewObjectID()
	}
	r.data = append(r.data, e)
	return nil
}
func (r *memApprovalRepo) List(_ context.Context, projectID string) ([]*Event, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	pid, _ := primitive.ObjectIDFromHex(projectID)
	out := make([]*Event, 0)
	for _, e := range r.data {
		if e.ProjectID == pid {
			out = append(out, e)
		}
	}
	return out, nil
}

// ---- 测试工具 ----

type testCtx struct {
	apSvc   *Service
	pSvc    *project.Service
	formSvc *formvalue.Service
	pRepo   *memProjectRepo
}

func setupCtx(t *testing.T) *testCtx {
	t.Helper()
	pRepo := newMemProjectRepo()
	oSvc := owner.NewService(newMemOwnerRepo())
	pSvc := project.NewService(pRepo, oSvc)
	tRepo := &memTmplRepo{}
	tSvc := formtemplate.NewService(tRepo)
	if err := tSvc.EnsureSeeded(context.Background()); err != nil {
		t.Fatalf("seed err=%v", err)
	}
	logSvc := changelog.NewService(newMemLogRepo())
	formSvc := formvalue.NewService(newMemValueRepo(), tSvc, pSvc, logSvc)
	snSvc := snapshot.NewService(newMemSnRepo())
	apSvc := NewService(newMemApprovalRepo(), pSvc, tSvc, formSvc, snSvc)
	return &testCtx{apSvc: apSvc, pSvc: pSvc, formSvc: formSvc, pRepo: pRepo}
}

func newProject(t *testing.T, c *testCtx) string {
	t.Helper()
	p, _, err := c.pSvc.Create(context.Background(), &project.CreateInput{
		ProjectName: "T", OwnerName: "O", ApplicantID: "u1", ApplicantName: "Alice",
	})
	if err != nil {
		t.Fatalf("create err=%v", err)
	}
	return p.ID.Hex()
}

func saveAllRequired(t *testing.T, c *testCtx, pid string) {
	t.Helper()
	_, err := c.formSvc.Save(context.Background(), &formvalue.SaveInput{
		ProjectID: pid, OperatorID: "u1", OperatorRole: "applicant",
		Changes: []formvalue.Change{
			{FieldKey: "base_info.project_overview", NewValue: "概况"},
			{FieldKey: "base_info.build_location", NewValue: "深圳"},
			{FieldKey: "base_info.build_period", NewValue: "12"},
			{FieldKey: "scale.description", NewValue: "规模"},
			{FieldKey: "investment.total_amount", NewValue: 1000},
			{FieldKey: "investment.fund_source", NewValue: "自筹"},
		},
	})
	if err != nil {
		t.Fatalf("save err=%v", err)
	}
}

// 用例：必填字段缺失提交被拒。
func TestSubmit_MissingRequired(t *testing.T) {
	c := setupCtx(t)
	pid := newProject(t, c)
	res, err := c.apSvc.Submit(context.Background(), pid, "u1", "Alice", "applicant")
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("expect ErrValidation, got %v", err)
	}
	if res == nil || len(res.MissingFields) == 0 {
		t.Fatalf("expect missing fields")
	}
}

// 用例：完整提交成功并迁移到 PENDING_REVIEW。
func TestSubmit_OK(t *testing.T) {
	c := setupCtx(t)
	pid := newProject(t, c)
	saveAllRequired(t, c, pid)
	res, err := c.apSvc.Submit(context.Background(), pid, "u1", "Alice", "applicant")
	if err != nil {
		t.Fatalf("submit err=%v", err)
	}
	if res.Status != project.StatusPendingReview {
		t.Fatalf("expect PENDING_REVIEW, got %s", res.Status)
	}
	if res.SnapshotID == "" {
		t.Fatalf("expect snapshot id")
	}
}

// 用例：审核通过 -> PENDING_APPROVE；终审通过 -> APPROVED 后写接口拒绝。
func TestApproveFlow_AllPass(t *testing.T) {
	c := setupCtx(t)
	pid := newProject(t, c)
	saveAllRequired(t, c, pid)
	if _, err := c.apSvc.Submit(context.Background(), pid, "u1", "Alice", "applicant"); err != nil {
		t.Fatalf("submit err=%v", err)
	}
	to, err := c.apSvc.Decide(context.Background(), pid, LevelReview, ActionApprove, "", "u2", "Bob", "reviewer")
	if err != nil {
		t.Fatalf("review err=%v", err)
	}
	if to != project.StatusPendingApprove {
		t.Fatalf("expect PENDING_APPROVE, got %s", to)
	}
	to, err = c.apSvc.Decide(context.Background(), pid, LevelFinal, ActionApprove, "", "u3", "Carol", "approver")
	if err != nil {
		t.Fatalf("final err=%v", err)
	}
	if to != project.StatusApproved {
		t.Fatalf("expect APPROVED, got %s", to)
	}
	// 已 APPROVED 后再次提交应 STATE_CONFLICT
	if _, err := c.apSvc.Submit(context.Background(), pid, "u1", "", "applicant"); !errors.Is(err, ErrStateConflict) {
		t.Fatalf("expect ErrStateConflict, got %v", err)
	}
}

// 用例：reject 必填 comment。
func TestReject_RequireComment(t *testing.T) {
	c := setupCtx(t)
	pid := newProject(t, c)
	saveAllRequired(t, c, pid)
	if _, err := c.apSvc.Submit(context.Background(), pid, "u1", "Alice", "applicant"); err != nil {
		t.Fatalf("submit err=%v", err)
	}
	if _, err := c.apSvc.Decide(context.Background(), pid, LevelReview, ActionReject, "", "u2", "", "reviewer"); !errors.Is(err, ErrCommentEmpty) {
		t.Fatalf("expect ErrCommentEmpty, got %v", err)
	}
	to, err := c.apSvc.Decide(context.Background(), pid, LevelReview, ActionReject, "格式不对", "u2", "", "reviewer")
	if err != nil {
		t.Fatalf("reject err=%v", err)
	}
	if to != project.StatusFormEditing {
		t.Fatalf("expect FORM_EDITING, got %s", to)
	}
	// 项目 LastPhase 应该是 REJECTED_REWORK
	p, _ := c.pSvc.GetByID(context.Background(), pid)
	if p.LastPhase != project.PhaseRejectedRework {
		t.Fatalf("expect last_phase=REJECTED_REWORK, got %s", p.LastPhase)
	}
}

// 用例：状态机非法迁移。在未 submit 时 Review 应返回 STATE_CONFLICT。
func TestDecide_StateConflict(t *testing.T) {
	c := setupCtx(t)
	pid := newProject(t, c)
	if _, err := c.apSvc.Decide(context.Background(), pid, LevelReview, ActionApprove, "", "u2", "", "reviewer"); !errors.Is(err, ErrStateConflict) {
		t.Fatalf("expect ErrStateConflict, got %v", err)
	}
}
