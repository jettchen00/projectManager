// Package project 项目主体：立项、状态机、列表。
package project

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// 状态枚举。
const (
	StatusDraft          = "DRAFT"
	StatusFormEditing    = "FORM_EDITING"
	StatusPendingReview  = "PENDING_REVIEW"
	StatusPendingApprove = "PENDING_APPROVE"
	StatusApproved       = "APPROVED"
	StatusRejected       = "REJECTED"
)

// 阶段：用于 ChangeLog.phase 区分驳回返工。
const (
	PhaseFormEditing    = "FORM_EDITING"
	PhaseRejectedRework = "REJECTED_REWORK"
)

// Project 项目实体。
type Project struct {
	ID              primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	ProjectCode     string             `bson:"project_code" json:"project_code"`
	ProjectName     string             `bson:"project_name" json:"project_name"`
	OwnerID         primitive.ObjectID `bson:"owner_id" json:"owner_id"`
	OwnerName       string             `bson:"owner_name" json:"owner_name"`
	Status          string             `bson:"status" json:"status"`
	CurrentRevision int32              `bson:"current_revision" json:"current_revision"`
	ApplicantID     string             `bson:"applicant_id" json:"applicant_id"`
	ApplicantName   string             `bson:"applicant_name" json:"applicant_name"`
	LastPhase       string             `bson:"last_phase" json:"last_phase"` // 上次审批后的阶段标识
	CreatedAt       time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt       time.Time          `bson:"updated_at" json:"updated_at"`
	ApprovedAt      *time.Time         `bson:"approved_at,omitempty" json:"approved_at,omitempty"`
}
