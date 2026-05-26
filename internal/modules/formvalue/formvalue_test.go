package formvalue

import (
	"context"
	"errors"
	"sync"
	"testing"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"projectManager/internal/modules/changelog"
	"projectManager/internal/modules/formtemplate"
	"projectManager/internal/modules/owner"
	"projectManager/internal/modules/project"
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

// memOwnerRepo
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

// memTmplRepo
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

// memChangeLogRepo
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
func (r *memChangeLogRepo) List(_ context.Context, q changelog.Query) ([]*changelog.ChangeLog, int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]*changelog.ChangeLog, 0)
	pid, _ := primitive.ObjectIDFromHex(q.ProjectID)
	for _, l := range r.data {
		if l.ProjectID != pid {
			continue
		}
		out = append(out, l)
	}
	return out, int64(len(out)), nil
}
func (r *memChangeLogRepo) ListByField(_ context.Context, projectID, fieldKey string) ([]*changelog.ChangeLog, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]*changelog.ChangeLog, 0)
	pid, _ := primitive.ObjectIDFromHex(projectID)
	for _, l := range r.data {
		if l.ProjectID == pid && l.FieldKey == fieldKey {
			out = append(out, l)
		}
	}
	return out, nil
}
func (r *memChangeLogRepo) ListByRevisionRange(_ context.Context, projectID string, from, to int32) ([]*changelog.ChangeLog, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]*changelog.ChangeLog, 0)
	pid, _ := primitive.ObjectIDFromHex(projectID)
	for _, l := range r.data {
		if l.ProjectID == pid && l.Revision > from && l.Revision <= to {
			out = append(out, l)
		}
	}
	return out, nil
}

// memValueRepo
type memValueRepo struct {
	mu   sync.Mutex
	data map[string]*FieldValue // key: pid|field_key
}

func newMemValueRepo() *memValueRepo { return &memValueRepo{data: map[string]*FieldValue{}} }
func (r *memValueRepo) GetAll(_ context.Context, projectID string) ([]*FieldValue, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	pid, err := primitive.ObjectIDFromHex(projectID)
	if err != nil {
		return nil, err
	}
	out := make([]*FieldValue, 0)
	for _, v := range r.data {
		if v.ProjectID == pid {
			cp := *v
			out = append(out, &cp)
		}
	}
	return out, nil
}
func (r *memValueRepo) Upsert(_ context.Context, fv *FieldValue) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.data[fv.ProjectID.Hex()+"|"+fv.FieldKey] = fv
	return nil
}

// ---- 测试构造 ----

type testCtx struct {
	formSvc *Service
	logRepo *memChangeLogRepo
	pRepo   *memProjectRepo
	pSvc    *project.Service
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
	logRepo := newMemLogRepo()
	logSvc := changelog.NewService(logRepo)
	formSvc := NewService(newMemValueRepo(), tSvc, pSvc, logSvc)
	return &testCtx{formSvc: formSvc, logRepo: logRepo, pRepo: pRepo, pSvc: pSvc}
}

func createProject(t *testing.T, c *testCtx) string {
	t.Helper()
	p, _, err := c.pSvc.Create(context.Background(), &project.CreateInput{
		ProjectName: "T1", OwnerName: "O1", ApplicantID: "u1", ApplicantName: "Alice",
	})
	if err != nil {
		t.Fatalf("create err=%v", err)
	}
	return p.ID.Hex()
}

// 用例：一次保存 N 字段产生 N 条 ChangeLog 且共享同一 revision。
func TestSave_SharedRevisionAcrossFields(t *testing.T) {
	c := setupCtx(t)
	pid := createProject(t, c)

	res, err := c.formSvc.Save(context.Background(), &SaveInput{
		ProjectID:    pid,
		OperatorID:   "u1",
		OperatorName: "Alice",
		OperatorRole: "applicant",
		Changes: []Change{
			{FieldKey: "base_info.project_overview", NewValue: "概况A"},
			{FieldKey: "base_info.build_location", NewValue: "深圳"},
			{FieldKey: "base_info.build_period", NewValue: "12个月"},
		},
	})
	if err != nil {
		t.Fatalf("save err=%v", err)
	}
	if res.AppliedCount != 3 {
		t.Fatalf("expect applied=3, got %d", res.AppliedCount)
	}
	if res.Revision != 1 {
		t.Fatalf("expect revision=1, got %d", res.Revision)
	}
	if len(c.logRepo.data) != 3 {
		t.Fatalf("expect 3 logs, got %d", len(c.logRepo.data))
	}
	for _, l := range c.logRepo.data {
		if l.Revision != 1 {
			t.Fatalf("log revision should share 1, got %d", l.Revision)
		}
	}
}

// 用例：字段值未变化不产生 ChangeLog。
func TestSave_NoOpWhenValueUnchanged(t *testing.T) {
	c := setupCtx(t)
	pid := createProject(t, c)

	in := &SaveInput{
		ProjectID: pid, OperatorID: "u1", OperatorRole: "applicant",
		Changes: []Change{{FieldKey: "base_info.build_location", NewValue: "深圳"}},
	}
	if _, err := c.formSvc.Save(context.Background(), in); err != nil {
		t.Fatalf("first save err=%v", err)
	}
	// 第二次相同值
	res, err := c.formSvc.Save(context.Background(), in)
	if err != nil {
		t.Fatalf("second save err=%v", err)
	}
	if res.AppliedCount != 0 {
		t.Fatalf("expect applied=0, got %d", res.AppliedCount)
	}
	// 项目 revision 应保持为 1（未自增）
	p, _ := c.pRepo.GetByID(context.Background(), pid)
	if p.CurrentRevision != 1 {
		t.Fatalf("expect revision=1, got %d", p.CurrentRevision)
	}
	if len(c.logRepo.data) != 1 {
		t.Fatalf("expect logs=1, got %d", len(c.logRepo.data))
	}
}

// 用例：字段权限不允许的角色被拒。
func TestSave_FieldRoleForbid(t *testing.T) {
	c := setupCtx(t)
	pid := createProject(t, c)
	_, err := c.formSvc.Save(context.Background(), &SaveInput{
		ProjectID: pid, OperatorID: "u9", OperatorRole: "viewer",
		Changes: []Change{{FieldKey: "base_info.build_location", NewValue: "X"}},
	})
	if !errors.Is(err, ErrFieldForbid) {
		t.Fatalf("expect ErrFieldForbid, got %v", err)
	}
}

// 用例：未知字段被拒。
func TestSave_UnknownField(t *testing.T) {
	c := setupCtx(t)
	pid := createProject(t, c)
	_, err := c.formSvc.Save(context.Background(), &SaveInput{
		ProjectID: pid, OperatorID: "u1", OperatorRole: "applicant",
		Changes: []Change{{FieldKey: "not.exist", NewValue: "X"}},
	})
	if !errors.Is(err, ErrFieldUnknown) {
		t.Fatalf("expect ErrFieldUnknown, got %v", err)
	}
}

// 用例：APPROVED 状态下写入被拒。
func TestSave_ApprovedReadOnly(t *testing.T) {
	c := setupCtx(t)
	pid := createProject(t, c)
	// 直接置为 APPROVED
	c.pRepo.mu.Lock()
	c.pRepo.data[pid].Status = project.StatusApproved
	c.pRepo.mu.Unlock()
	_, err := c.formSvc.Save(context.Background(), &SaveInput{
		ProjectID: pid, OperatorID: "u1", OperatorRole: "applicant",
		Changes: []Change{{FieldKey: "base_info.build_location", NewValue: "X"}},
	})
	if !errors.Is(err, ErrStateConflict) {
		t.Fatalf("expect ErrStateConflict, got %v", err)
	}
}

// 用例：驳回返工后 phase=REJECTED_REWORK。
func TestSave_PhaseRejectedRework(t *testing.T) {
	c := setupCtx(t)
	pid := createProject(t, c)
	if err := c.pSvc.UpdateLastPhase(context.Background(), pid, project.PhaseRejectedRework); err != nil {
		t.Fatalf("set phase err=%v", err)
	}
	if _, err := c.formSvc.Save(context.Background(), &SaveInput{
		ProjectID: pid, OperatorID: "u1", OperatorRole: "applicant",
		Changes: []Change{{FieldKey: "base_info.build_location", NewValue: "新地点"}},
	}); err != nil {
		t.Fatalf("save err=%v", err)
	}
	if len(c.logRepo.data) != 1 {
		t.Fatalf("expect 1 log, got %d", len(c.logRepo.data))
	}
	if c.logRepo.data[0].Phase != project.PhaseRejectedRework {
		t.Fatalf("expect phase=REJECTED_REWORK, got %s", c.logRepo.data[0].Phase)
	}
}
