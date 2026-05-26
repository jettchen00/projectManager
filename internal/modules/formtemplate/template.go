// Package formtemplate 项目表格模板（字段配置）。
package formtemplate

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

// 字段类型枚举。
const (
	TypeText        = "text"
	TypeTextarea    = "textarea"
	TypeNumber      = "number"
	TypeMoney       = "money"
	TypeDate        = "date"
	TypeSelect      = "select"
	TypeMultiselect = "multiselect"
	TypeFile        = "file"
	TypeTable       = "table"
)

// Field 模板字段。
type Field struct {
	FieldKey      string   `bson:"field_key" json:"field_key"`
	Label         string   `bson:"label" json:"label"`
	Type          string   `bson:"type" json:"type"`
	Required      bool     `bson:"required" json:"required"`
	EditableRoles []string `bson:"editable_roles" json:"editable_roles"`
	Options       []string `bson:"options,omitempty" json:"options,omitempty"`
	Placeholder   string   `bson:"placeholder,omitempty" json:"placeholder,omitempty"`
}

// Section 模板分组。
type Section struct {
	SectionKey   string  `bson:"section_key" json:"section_key"`
	SectionLabel string  `bson:"section_label" json:"section_label"`
	Fields       []Field `bson:"fields" json:"fields"`
}

// Template 完整模板。
type Template struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Version   int32              `bson:"version" json:"version"`
	Active    bool               `bson:"active" json:"active"`
	Sections  []Section          `bson:"sections" json:"sections"`
	CreatedAt time.Time          `bson:"created_at" json:"created_at"`
}

// Repo 模板存储。
type Repo interface {
	GetActive(ctx context.Context) (*Template, error)
	InsertIfEmpty(ctx context.Context, t *Template) error
}

// MongoRepo Mongo 实现。
type MongoRepo struct{ store *mstore.Store }

// NewMongoRepo 构造。
func NewMongoRepo(s *mstore.Store) *MongoRepo { return &MongoRepo{store: s} }

func (r *MongoRepo) coll() *mongo.Collection { return r.store.Coll(mstore.CollFormTemplates) }

// GetActive 取当前生效模板。
func (r *MongoRepo) GetActive(ctx context.Context) (*Template, error) {
	var t Template
	err := r.coll().FindOne(ctx, bson.M{"active": true}).Decode(&t)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		pmlog.Errorf("template GetActive err=%v", err)
		return nil, err
	}
	return &t, nil
}

// InsertIfEmpty 集合空时写入默认模板。
func (r *MongoRepo) InsertIfEmpty(ctx context.Context, t *Template) error {
	count, err := r.coll().CountDocuments(ctx, bson.M{}, options.Count().SetLimit(1))
	if err != nil {
		pmlog.Errorf("template Count err=%v", err)
		return err
	}
	if count > 0 {
		return nil
	}
	t.CreatedAt = time.Now()
	if _, err := r.coll().InsertOne(ctx, t); err != nil {
		pmlog.Errorf("template Insert err=%v", err)
		return err
	}
	pmlog.Infof("template seeded version=%d", t.Version)
	return nil
}

// Service 模板服务。
type Service struct{ repo Repo }

// NewService 构造。
func NewService(r Repo) *Service { return &Service{repo: r} }

// EnsureSeeded 启动时调用，写入默认模板。
func (s *Service) EnsureSeeded(ctx context.Context) error {
	return s.repo.InsertIfEmpty(ctx, DefaultTemplate())
}

// GetActive 当前模板。
func (s *Service) GetActive(ctx context.Context) (*Template, error) {
	return s.repo.GetActive(ctx)
}

// FieldMap 把模板转为 field_key -> Field 的查找表。
func FieldMap(t *Template) map[string]Field {
	m := make(map[string]Field)
	if t == nil {
		return m
	}
	for _, sec := range t.Sections {
		for _, f := range sec.Fields {
			m[f.FieldKey] = f
		}
	}
	return m
}

// DefaultTemplate 内置默认模板（来源：requirement_template.docx 的高层抽象）。
func DefaultTemplate() *Template {
	editable := []string{"applicant", "editor", "admin"}
	return &Template{
		Version: 1,
		Active:  true,
		Sections: []Section{
			{
				SectionKey:   "base_info",
				SectionLabel: "项目基本信息",
				Fields: []Field{
					{FieldKey: "base_info.project_overview", Label: "项目概况", Type: TypeTextarea, Required: true, EditableRoles: editable},
					{FieldKey: "base_info.build_location", Label: "建设地点", Type: TypeText, Required: true, EditableRoles: editable},
					{FieldKey: "base_info.build_period", Label: "建设周期", Type: TypeText, Required: true, EditableRoles: editable},
				},
			},
			{
				SectionKey:   "scale",
				SectionLabel: "建设规模",
				Fields: []Field{
					{FieldKey: "scale.description", Label: "建设规模说明", Type: TypeTextarea, Required: true, EditableRoles: editable},
					{FieldKey: "scale.area_sqm", Label: "占地面积(平方米)", Type: TypeNumber, Required: false, EditableRoles: editable},
				},
			},
			{
				SectionKey:   "investment",
				SectionLabel: "投资估算",
				Fields: []Field{
					{FieldKey: "investment.total_amount", Label: "总投资(元)", Type: TypeMoney, Required: true, EditableRoles: editable},
					{FieldKey: "investment.fund_source", Label: "资金来源", Type: TypeText, Required: true, EditableRoles: editable},
				},
			},
			{
				SectionKey:   "tech",
				SectionLabel: "技术方案",
				Fields: []Field{
					{FieldKey: "tech.scheme", Label: "技术方案", Type: TypeTextarea, Required: false, EditableRoles: editable},
				},
			},
		},
	}
}
