package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ProjectRepository struct {
	ID            string    `gorm:"type:text;primaryKey" json:"id"`
	ProjectID     string    `gorm:"type:text;not null;index" json:"projectId"`
	Project       *Project  `json:"-"`
	Name          string    `gorm:"type:text;not null" json:"name"`
	GitURL        string    `gorm:"type:text;not null" json:"gitUrl"`
	DefaultBranch string    `gorm:"type:text;default:'main';not null" json:"defaultBranch"`
	CreatedAt     time.Time `gorm:"default:current_timestamp" json:"createdAt"`
	UpdatedAt     time.Time `gorm:"default:current_timestamp" json:"updatedAt"`
}

func (m *ProjectRepository) BeforeCreate(tx *gorm.DB) (err error) {
	if m.ID == "" {
		m.ID = uuid.NewString()
	}
	return
}

type ProjectMemberRole string

const (
	ProjectMemberRoleOwner  ProjectMemberRole = "OWNER"
	ProjectMemberRoleEditor ProjectMemberRole = "EDITOR"
	ProjectMemberRoleViewer ProjectMemberRole = "VIEWER"
)

type ProjectMemberStatus string

const (
	ProjectMemberStatusPending  ProjectMemberStatus = "PENDING"
	ProjectMemberStatusAccepted ProjectMemberStatus = "ACCEPTED"
	ProjectMemberStatusRejected ProjectMemberStatus = "REJECTED"
)

type ProjectMember struct {
	ID              string              `gorm:"type:text;primaryKey" json:"id"`
	ProjectID       string              `gorm:"type:text;not null;index" json:"projectId"`
	Project         *Project            `json:"-"`
	UserID          string              `gorm:"type:text;not null;index" json:"userId"`
	User            *User               `json:"-"`
	Role            ProjectMemberRole   `gorm:"type:text;not null" json:"role"`
	Status          ProjectMemberStatus `gorm:"type:text;not null;default:'PENDING'" json:"status"`
	InvitedByUserID *string             `gorm:"type:text" json:"invitedBy"`
	InvitedAt       time.Time           `gorm:"default:current_timestamp" json:"invitedAt"`
	AcceptedAt      *time.Time          `json:"acceptedAt"`
	CreatedAt       time.Time           `gorm:"default:current_timestamp" json:"createdAt"`
	UpdatedAt       time.Time           `gorm:"default:current_timestamp" json:"updatedAt"`
}

func (m *ProjectMember) BeforeCreate(tx *gorm.DB) (err error) {
	if m.ID == "" {
		m.ID = uuid.NewString()
	}
	return
}

type WorkspaceType string

const (
	WorkspaceTypeCanonical WorkspaceType = "CANONICAL"
	WorkspaceTypeFork      WorkspaceType = "FORK"
)

type WorkspaceStatus string

const (
	WorkspaceStatusActive   WorkspaceStatus = "ACTIVE"
	WorkspaceStatusIdle     WorkspaceStatus = "IDLE"
	WorkspaceStatusStopped  WorkspaceStatus = "STOPPED"
	WorkspaceStatusArchived WorkspaceStatus = "ARCHIVED"
	WorkspaceStatusDeleted  WorkspaceStatus = "DELETED"
)

type Workspace struct {
	ID          string          `gorm:"type:text;primaryKey" json:"id"`
	ProjectID   string          `gorm:"type:text;not null;index" json:"projectId"`
	Project     *Project        `json:"-"`
	OwnerUserID string          `gorm:"type:text;not null;index" json:"ownerUserId"`
	Type        WorkspaceType   `gorm:"type:text;not null" json:"type"`
	BaseCommit  string          `gorm:"type:text" json:"baseCommit"`
	Status      WorkspaceStatus `gorm:"type:text;not null;default:'ACTIVE'" json:"status"`
	CreatedAt   time.Time       `gorm:"default:current_timestamp" json:"createdAt"`
	UpdatedAt   time.Time       `gorm:"default:current_timestamp" json:"updatedAt"`

	// Link to the environment representing the running instance of this workspace
	EnvironmentID *string      `gorm:"type:text" json:"environmentId"`
	Environment   *Environment `json:"environment"`
}

func (m *Workspace) BeforeCreate(tx *gorm.DB) (err error) {
	if m.ID == "" {
		m.ID = uuid.NewString()
	}
	return
}

type WorkspaceRepository struct {
	ID                  string             `gorm:"type:text;primaryKey" json:"id"`
	WorkspaceID         string             `gorm:"type:text;not null;index" json:"workspaceId"`
	Workspace           *Workspace         `json:"-"`
	ProjectRepositoryID string             `gorm:"type:text;not null;index" json:"projectRepositoryId"`
	ProjectRepository   *ProjectRepository `json:"-"`
	Branch              string             `gorm:"type:text;not null" json:"branch"`
	BaseCommit          string             `gorm:"type:text" json:"baseCommit"`
	CurrentCommit       string             `gorm:"type:text" json:"currentCommit"`
	WorkingDirectory    string             `gorm:"type:text" json:"workingDirectory"`
	Status              string             `gorm:"type:text;not null;default:'SYNCED'" json:"status"`
	CreatedAt           time.Time          `gorm:"default:current_timestamp" json:"createdAt"`
	UpdatedAt           time.Time          `gorm:"default:current_timestamp" json:"updatedAt"`
}

func (m *WorkspaceRepository) BeforeCreate(tx *gorm.DB) (err error) {
	if m.ID == "" {
		m.ID = uuid.NewString()
	}
	return
}

type ChangeRequestStatus string

const (
	ChangeRequestStatusOpen       ChangeRequestStatus = "OPEN"
	ChangeRequestStatusApproved   ChangeRequestStatus = "APPROVED"
	ChangeRequestStatusRejected   ChangeRequestStatus = "REJECTED"
	ChangeRequestStatusMerged     ChangeRequestStatus = "MERGED"
	ChangeRequestStatusClosed     ChangeRequestStatus = "CLOSED"
	ChangeRequestStatusConflicted ChangeRequestStatus = "CONFLICTED"
)

type ChangeRequest struct {
	ID              string              `gorm:"type:text;primaryKey" json:"id"`
	ProjectID       string              `gorm:"type:text;not null;index" json:"projectId"`
	Project         *Project            `json:"-"`
	WorkspaceID     string              `gorm:"type:text;not null;index" json:"workspaceId"`
	Workspace       *Workspace          `json:"-"`
	CreatedByUserID string              `gorm:"type:text;not null;index" json:"createdByUserId"`
	CreatedByUser   *User               `json:"-"`
	Status          ChangeRequestStatus `gorm:"type:text;not null;default:'OPEN'" json:"status"`
	Title           string              `gorm:"type:text;not null" json:"title"`
	Description     string              `gorm:"type:text" json:"description"`
	CreatedAt       time.Time           `gorm:"default:current_timestamp" json:"createdAt"`
	UpdatedAt       time.Time           `gorm:"default:current_timestamp" json:"updatedAt"`
}

func (m *ChangeRequest) BeforeCreate(tx *gorm.DB) (err error) {
	if m.ID == "" {
		m.ID = uuid.NewString()
	}
	return
}
