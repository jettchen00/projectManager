package httpapi

import (
	"net/http"

	"projectManager/internal/errcode"
	"projectManager/internal/modules/owner"
)

// OwnerController 业主单位控制器。
type OwnerController struct {
	svc *owner.Service
}

// NewOwnerController 构造。
func NewOwnerController(s *owner.Service) *OwnerController { return &OwnerController{svc: s} }

// List 列表。
func (c *OwnerController) List(w http.ResponseWriter, r *http.Request) {
	keyword := r.URL.Query().Get("keyword")
	limit := queryInt(r, "limit", 50)
	list, err := c.svc.List(r.Context(), keyword, limit)
	if err != nil {
		errcode.WriteErr(w, errcode.CodeDBError, nil)
		return
	}
	items := make([]map[string]interface{}, 0, len(list))
	for _, o := range list {
		items = append(items, map[string]interface{}{
			"id":            o.ID.Hex(),
			"name":          o.Name,
			"owner_type":    o.OwnerType,
			"contact_name":  o.ContactName,
			"contact_phone": o.ContactPhone,
			"contact_email": o.ContactEmail,
			"address":       o.Address,
			"created_at":    o.CreatedAt,
		})
	}
	errcode.WriteOK(w, map[string]interface{}{"items": items})
}
