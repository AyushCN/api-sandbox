package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type EnvironmentStatus string

const (
	StatusIdle     EnvironmentStatus = "IDLE"
	StatusBuilding EnvironmentStatus = "BUILDING"
	StatusRunning  EnvironmentStatus = "RUNNING"
	StatusStopped  EnvironmentStatus = "STOPPED"
	StatusFailed   EnvironmentStatus = "FAILED"
)

type User struct {
	ID        string    `gorm:"type:text;primaryKey" json:"id"`
	Email     string    `gorm:"type:text;unique;not null" json:"email"`
	Username  string    `gorm:"type:text;uniqueIndex" json:"username"`
	Password         string     `gorm:"type:text;not null" json:"-"` // never return password to client
	IsEmailVerified  bool       `gorm:"default:false" json:"isEmailVerified"`
	VerificationCode  string     `gorm:"type:text" json:"-"`
	VerificationExp   *time.Time `gorm:"type:timestamp" json:"-"`
	ResetPasswordCode string     `gorm:"type:text" json:"-"`
	ResetPasswordExp  *time.Time `gorm:"type:timestamp" json:"-"`
	MaxEnvironments   int        `gorm:"default:5" json:"maxEnvironments"`
	MaxBuildsPerHour  int        `gorm:"default:10" json:"maxBuildsPerHour"`
	Bio               string     `gorm:"type:text" json:"bio"`
	Pronouns          string     `gorm:"type:text" json:"pronouns"`
	Location          string     `gorm:"type:text" json:"location"`
	Website           string     `gorm:"type:text" json:"website"`
	Twitter           string     `gorm:"type:text" json:"twitter"`
	Github            string     `gorm:"type:text" json:"github"`
	CreatedAt         time.Time  `gorm:"default:current_timestamp" json:"createdAt"`
	UpdatedAt        time.Time  `gorm:"default:current_timestamp" json:"updatedAt"`
}

func (u *User) BeforeCreate(tx *gorm.DB) (err error) {
	if u.ID == "" {
		u.ID = uuid.NewString()
	}
	if u.Username == "" {
		u.Username = "user_" + u.ID[:8]
	}
	return
}

type OrganizationRole string

const (
	RoleAdmin  OrganizationRole = "ADMIN"
	RoleMember OrganizationRole = "MEMBER"
)

type Organization struct {
	ID        string    `gorm:"type:text;primaryKey" json:"id"`
	Name      string    `gorm:"type:text;not null" json:"name"`
	CreatedAt time.Time `gorm:"default:current_timestamp" json:"createdAt"`
	UpdatedAt time.Time `gorm:"default:current_timestamp" json:"updatedAt"`
}

func (o *Organization) BeforeCreate(tx *gorm.DB) (err error) {
	if o.ID == "" {
		o.ID = uuid.NewString()
	}
	return
}

type OrganizationMember struct {
	ID             string           `gorm:"type:text;primaryKey" json:"id"`
	OrganizationID string           `gorm:"type:text;not null;index" json:"organizationId"`
	Organization   Organization     `json:"-"`
	UserID         string           `gorm:"type:text;not null;index" json:"userId"`
	User           User             `json:"-"`
	Role           OrganizationRole `gorm:"type:text;default:MEMBER;not null" json:"role"`
	CreatedAt      time.Time        `gorm:"default:current_timestamp" json:"createdAt"`
}

func (m *OrganizationMember) BeforeCreate(tx *gorm.DB) (err error) {
	if m.ID == "" {
		m.ID = uuid.NewString()
	}
	return
}

type Project struct {
	ID                  string                `gorm:"type:text;primaryKey" json:"id"`
	Name                string                `gorm:"type:text;not null" json:"name"`
	Description         string                `gorm:"type:text" json:"description"`
	CreatedByUserID     string                `gorm:"type:text;not null" json:"createdBy"`
	CreatedByUser       User                  `json:"-"`
	OwnerOrganizationID string                `gorm:"type:text;not null;index" json:"ownerOrganizationId"`
	OwnerOrganization   Organization          `json:"-"`
	Collaborators       []ProjectCollaborator `json:"collaborators,omitempty"`
	Environments        []Environment         `json:"environments,omitempty"`
	IsPublic            bool                  `gorm:"default:false" json:"isPublic"`
	CreatedAt           time.Time             `gorm:"default:current_timestamp" json:"createdAt"`
	UpdatedAt           time.Time             `gorm:"default:current_timestamp" json:"updatedAt"`
}

func (p *Project) BeforeCreate(tx *gorm.DB) (err error) {
	if p.ID == "" {
		p.ID = uuid.NewString()
	}
	return
}

type ProjectRole string

const (
	ProjectRoleOwner        ProjectRole = "OWNER"
	ProjectRoleAdmin        ProjectRole = "ADMIN"
	ProjectRoleCollaborator ProjectRole = "COLLABORATOR"
	ProjectRoleViewer       ProjectRole = "VIEWER"
)

type ProjectCollaborator struct {
	ID              string      `gorm:"type:text;primaryKey" json:"id"`
	ProjectID       string      `gorm:"type:text;not null;index" json:"projectId"`
	Project         Project     `json:"-"`
	UserID          string      `gorm:"type:text;not null;index" json:"userId"`
	User            User        `json:"user"`
	UserOrg         string      `gorm:"type:text;index" json:"userOrganization"` // For reference
	Role            ProjectRole `gorm:"type:text;default:'COLLABORATOR';not null" json:"role"`
	InvitedByUserID string      `gorm:"type:text" json:"invitedBy"`
	InvitedAt       time.Time   `gorm:"default:current_timestamp" json:"invitedAt"`
	AcceptedAt      *time.Time  `json:"acceptedAt"` // NULL until user accepts invite
}

func (pc *ProjectCollaborator) BeforeCreate(tx *gorm.DB) (err error) {
	if pc.ID == "" {
		pc.ID = uuid.NewString()
	}
	return
}

type Environment struct {
	ID             string            `gorm:"type:text;primaryKey" json:"id"`
	ProjectID      string            `gorm:"type:text;index" json:"projectId"`
	Project        *Project          `json:"-"`
	OrganizationID string            `gorm:"type:text;index" json:"organizationId"` // Legacy reference
	Organization   *Organization     `json:"-"`
	UserID         string            `gorm:"type:text;not null" json:"userId"`      // Creator
	User           User              `json:"-"`
	Name           string            `gorm:"type:text;not null" json:"name"`
	GitURL         string            `gorm:"type:text;not null" json:"gitUrl"`
	GithubBranch   string            `gorm:"type:text;default:main;not null" json:"githubBranch"`
	Status         EnvironmentStatus `gorm:"type:text;default:IDLE;not null" json:"status"`
	PublicURL      *string           `gorm:"type:text" json:"publicUrl"`
	UserProvidedDBURL *string        `gorm:"type:text" json:"userProvidedDbUrl"`
	ContainerID    *string           `gorm:"type:text" json:"containerId"`
	Port           *int              `gorm:"type:integer" json:"port"`
	CreatedAt      time.Time         `gorm:"default:current_timestamp" json:"createdAt"`
	UpdatedAt      time.Time         `gorm:"default:current_timestamp" json:"updatedAt"`
	ExpiresAt      *time.Time        `gorm:"type:timestamp(3) without time zone" json:"expiresAt"`

	Logs    []Log    `gorm:"constraint:OnDelete:CASCADE;" json:"logs,omitempty"`
	Metrics []Metric `gorm:"constraint:OnDelete:CASCADE;" json:"metrics,omitempty"`
}

func (e *Environment) BeforeCreate(tx *gorm.DB) (err error) {
	if e.ID == "" {
		// Use a simple UUID (in Prisma we used cuid, but uuid is fine for Go, or we can use segmentio/ksuid)
		// Since we didn't add the uuid package to go get, I'll use a simple fallback or just rely on postgres gen_random_uuid()
		// Actually let's use the standard google/uuid since it's very common.
		e.ID = uuid.NewString()
	}
	return
}

type EnvironmentRole string

const (
	EnvRoleAdmin  EnvironmentRole = "ADMIN"
	EnvRoleMember EnvironmentRole = "MEMBER"
	EnvRoleViewer EnvironmentRole = "VIEWER"
)

type EnvironmentMember struct {
	ID            string          `gorm:"type:text;primaryKey" json:"id"`
	EnvironmentID string          `gorm:"type:text;not null;index" json:"environmentId"`
	Environment   Environment     `json:"-"`
	UserID        string          `gorm:"type:text;not null;index" json:"userId"`
	User          User            `json:"-"`
	Role          EnvironmentRole `gorm:"type:text;default:MEMBER;not null" json:"role"`
	CreatedAt     time.Time       `gorm:"default:current_timestamp" json:"createdAt"`
}

func (m *EnvironmentMember) BeforeCreate(tx *gorm.DB) (err error) {
	if m.ID == "" {
		m.ID = uuid.NewString()
	}
	return
}

type LogLevel string

const (
	LogLevelInfo  LogLevel = "info"
	LogLevelError LogLevel = "error"
	LogLevelWarn  LogLevel = "warn"
)

type Log struct {
	ID            string      `gorm:"type:text;primaryKey" json:"id"`
	EnvironmentID string      `gorm:"type:text;not null" json:"environmentId"`
	Environment   Environment `json:"-"`
	Message       string      `gorm:"type:text;not null" json:"message"`
	Level         LogLevel    `gorm:"type:text;default:info;not null" json:"level"`
	Timestamp     time.Time   `gorm:"default:current_timestamp" json:"timestamp"`
}

func (l *Log) BeforeCreate(tx *gorm.DB) (err error) {
	if l.ID == "" {
		l.ID = uuid.NewString()
	}
	return
}

type Metric struct {
	ID            string      `gorm:"type:text;primaryKey" json:"id"`
	EnvironmentID string      `gorm:"type:text;not null" json:"environmentId"`
	Environment   Environment `json:"-"`
	CpuUsage      float64     `gorm:"type:double precision;not null" json:"cpuUsage"`
	MemoryUsage   float64     `gorm:"type:double precision;not null" json:"memoryUsage"`
	Timestamp     time.Time   `gorm:"default:current_timestamp" json:"timestamp"`
}

func (m *Metric) BeforeCreate(tx *gorm.DB) (err error) {
	if m.ID == "" {
		m.ID = uuid.NewString()
	}
	return
}

type AuditLog struct {
	ID        string    `gorm:"type:text;primaryKey" json:"id"`
	UserID    string    `gorm:"type:text;not null" json:"userId"`
	User      User      `json:"-"`
	Action    string    `gorm:"type:text;not null" json:"action"`
	Resource  string    `gorm:"type:text" json:"resource"`
	IPAddress string    `gorm:"type:text" json:"ipAddress"`
	Timestamp time.Time `gorm:"default:current_timestamp" json:"timestamp"`
}

func (a *AuditLog) BeforeCreate(tx *gorm.DB) (err error) {
	if a.ID == "" {
		a.ID = uuid.NewString()
	}
	return
}
