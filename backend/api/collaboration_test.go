package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/api-sandbox/backend/db"
	"github.com/api-sandbox/backend/models"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupTestDBForCollaboration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var err error
	db.DB, err = gorm.Open(sqlite.Open(fmt.Sprintf("file:collab_%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("Failed to connect to in-memory SQLite: %v", err)
	}

	err = db.DB.AutoMigrate(
		&models.User{},
		&models.Project{},
		&models.ProjectMember{},
		&models.CollaborationInvitation{},
		&models.Workspace{},
		&models.Environment{},
		&models.Log{},
		&models.Metric{},
	)
	if err != nil {
		t.Fatalf("AutoMigrate failed: %v", err)
	}
}

func setupCollaborationRouter() *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())

	// Custom auth middleware for testing that reads X-Test-User header
	authMiddleware := func() gin.HandlerFunc {
		return func(c *gin.Context) {
			userID := c.GetHeader("X-Test-User")
			if userID == "" {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
				return
			}
			c.Set("userId", userID)
			c.Next()
		}
	}

	apiGroup := r.Group("/api")
	apiGroup.Use(authMiddleware())
	{
		apiGroup.GET("/user/invitations", GetUserInvitations)
		apiGroup.POST("/invitations/:id/accept", AcceptInvitation)
		apiGroup.POST("/invitations/:id/reject", RejectInvitation)

		projects := apiGroup.Group("/projects")
		{
			projects.GET("", GetUserProjects)
			projects.POST("", CreateProject)

			projectDetail := projects.Group("/:projectId")
			{
				projectDetail.POST("/invites/accept", AcceptProjectInvite)
				projectDetail.POST("/invites/decline", DeclineProjectInvite)

				projectDetail.Use(AuthorizeProjectMemberAccess(models.ProjectMemberRoleViewer))
				{
					projectDetail.GET("", GetProject)
					projectDetail.DELETE("", AuthorizeProjectMemberAccess(models.ProjectMemberRoleOwner), DeleteProject)
					projectDetail.POST("/transfer", AuthorizeProjectMemberAccess(models.ProjectMemberRoleOwner), TransferProjectOwnership)
					projectDetail.POST("/invite", AuthorizeProjectMemberAccess(models.ProjectMemberRoleOwner), InviteToProject)
					projectDetail.POST("/invitations", AuthorizeProjectMemberAccess(models.ProjectMemberRoleOwner), InviteToProject)
					projectDetail.DELETE("/invitations/:id", AuthorizeProjectMemberAccess(models.ProjectMemberRoleOwner), RevokeInvitation)
					projectDetail.DELETE("/collaborators/:userId", RemoveCollaborator)
					projectDetail.PUT("/collaborators/:userId/role", AuthorizeProjectMemberAccess(models.ProjectMemberRoleOwner), UpdateMemberRole)
				}
			}
		}

		environments := apiGroup.Group("/environments")
		{
			environments.GET("/:id/files", GetWorkspaceFiles)
			environments.POST("/:id/files/content", UpdateWorkspaceFileContent)
			environments.POST("/:id/restart", RestartEnvironment)
			environments.DELETE("/:id", DeleteEnvironment)
		}
	}

	return r
}

func TestDefaultProjectCannotBeShared(t *testing.T) {
	setupTestDBForCollaboration(t)
	r := setupCollaborationRouter()

	owner := models.User{Email: "owner@test.com", Username: "owner"}
	invitee := models.User{Email: "invitee@test.com", Username: "invitee"}
	db.DB.Create(&owner)
	db.DB.Create(&invitee)

	// Create Default Project for owner
	defaultProj := models.Project{
		Name:            "Personal Space",
		CreatedByUserID: owner.ID,
		IsDefault:       true,
	}
	db.DB.Create(&defaultProj)
	db.DB.Create(&models.ProjectMember{
		ProjectID: defaultProj.ID,
		UserID:    owner.ID,
		Role:      models.ProjectMemberRoleOwner,
		Status:    models.ProjectMemberStatusAccepted,
	})

	// Attempt to invite to Default Project
	payload := `{"identifier":"invitee@test.com","role":"EDITOR"}`
	req, _ := http.NewRequest("POST", "/api/projects/"+defaultProj.ID+"/invite", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-User", owner.ID)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("Expected 403 Forbidden when sharing Default Project, got %d: %s", w.Code, w.Body.String())
	}

	// Attempt to delete Default Project
	delReq, _ := http.NewRequest("DELETE", "/api/projects/"+defaultProj.ID, nil)
	delReq.Header.Set("X-Test-User", owner.ID)
	wDel := httptest.NewRecorder()
	r.ServeHTTP(wDel, delReq)

	if wDel.Code != http.StatusForbidden {
		t.Fatalf("Expected 403 Forbidden when deleting Default Project, got %d", wDel.Code)
	}
}

func TestShareableProjectCanBeShared(t *testing.T) {
	setupTestDBForCollaboration(t)
	r := setupCollaborationRouter()

	owner := models.User{Email: "owner2@test.com", Username: "owner2"}
	invitee := models.User{Email: "invitee2@test.com", Username: "invitee2"}
	db.DB.Create(&owner)
	db.DB.Create(&invitee)

	shareableProj := models.Project{
		Name:            "Microservices API",
		CreatedByUserID: owner.ID,
		IsDefault:       false,
	}
	db.DB.Create(&shareableProj)
	db.DB.Create(&models.ProjectMember{
		ProjectID: shareableProj.ID,
		UserID:    owner.ID,
		Role:      models.ProjectMemberRoleOwner,
		Status:    models.ProjectMemberStatusAccepted,
	})

	payload := `{"identifier":"invitee2@test.com","role":"EDITOR"}`
	req, _ := http.NewRequest("POST", "/api/projects/"+shareableProj.ID+"/invite", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-User", owner.ID)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK when inviting to shareable project, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Invitation models.CollaborationInvitation `json:"invitation"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Invitation.Status != models.InvitePending {
		t.Fatalf("Expected invitation status PENDING, got %s", resp.Invitation.Status)
	}
	if resp.Invitation.Role != models.ProjectMemberRoleEditor {
		t.Fatalf("Expected invitation role EDITOR, got %s", resp.Invitation.Role)
	}
}

func TestSelfInviteBlocked(t *testing.T) {
	setupTestDBForCollaboration(t)
	r := setupCollaborationRouter()

	owner := models.User{Email: "owner3@test.com", Username: "owner3"}
	db.DB.Create(&owner)

	shareableProj := models.Project{
		Name:            "Team Project",
		CreatedByUserID: owner.ID,
		IsDefault:       false,
	}
	db.DB.Create(&shareableProj)
	db.DB.Create(&models.ProjectMember{
		ProjectID: shareableProj.ID,
		UserID:    owner.ID,
		Role:      models.ProjectMemberRoleOwner,
		Status:    models.ProjectMemberStatusAccepted,
	})

	payload := `{"identifier":"owner3@test.com","role":"EDITOR"}`
	req, _ := http.NewRequest("POST", "/api/projects/"+shareableProj.ID+"/invite", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-User", owner.ID)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("Expected 400 Bad Request for self-invitation, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDuplicateInviteAndAlreadyMemberBlocked(t *testing.T) {
	setupTestDBForCollaboration(t)
	r := setupCollaborationRouter()

	owner := models.User{Email: "owner4@test.com", Username: "owner4"}
	userB := models.User{Email: "userb@test.com", Username: "userb"}
	db.DB.Create(&owner)
	db.DB.Create(&userB)

	shareableProj := models.Project{
		Name:            "Team Project",
		CreatedByUserID: owner.ID,
		IsDefault:       false,
	}
	db.DB.Create(&shareableProj)
	db.DB.Create(&models.ProjectMember{
		ProjectID: shareableProj.ID,
		UserID:    owner.ID,
		Role:      models.ProjectMemberRoleOwner,
		Status:    models.ProjectMemberStatusAccepted,
	})

	// First invite
	payload := `{"identifier":"userb@test.com","role":"EDITOR"}`
	req1, _ := http.NewRequest("POST", "/api/projects/"+shareableProj.ID+"/invite", bytes.NewBufferString(payload))
	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set("X-Test-User", owner.ID)
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)
	if w1.Code != http.StatusOK {
		t.Fatalf("Expected 200 for first invite, got %d", w1.Code)
	}

	// Duplicate invite while pending
	req2, _ := http.NewRequest("POST", "/api/projects/"+shareableProj.ID+"/invite", bytes.NewBufferString(payload))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("X-Test-User", owner.ID)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusConflict {
		t.Fatalf("Expected 409 Conflict for duplicate pending invite, got %d: %s", w2.Code, w2.Body.String())
	}

	// User B accepts
	var invite models.CollaborationInvitation
	db.DB.First(&invite, "project_id = ? AND invitee_id = ?", shareableProj.ID, userB.ID)
	accReq, _ := http.NewRequest("POST", "/api/invitations/"+invite.ID+"/accept", nil)
	accReq.Header.Set("X-Test-User", userB.ID)
	wAcc := httptest.NewRecorder()
	r.ServeHTTP(wAcc, accReq)
	if wAcc.Code != http.StatusOK {
		t.Fatalf("Expected 200 on acceptance, got %d", wAcc.Code)
	}

	// Invite user who is already a member
	req3, _ := http.NewRequest("POST", "/api/projects/"+shareableProj.ID+"/invite", bytes.NewBufferString(payload))
	req3.Header.Set("Content-Type", "application/json")
	req3.Header.Set("X-Test-User", owner.ID)
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, req3)
	if w3.Code != http.StatusConflict {
		t.Fatalf("Expected 409 Conflict for already accepted member, got %d: %s", w3.Code, w3.Body.String())
	}
}

func TestPendingInvitationDoesNotGrantAccess(t *testing.T) {
	setupTestDBForCollaboration(t)
	r := setupCollaborationRouter()

	owner := models.User{Email: "owner5@test.com", Username: "owner5"}
	userB := models.User{Email: "userb5@test.com", Username: "userb5"}
	db.DB.Create(&owner)
	db.DB.Create(&userB)

	shareableProj := models.Project{
		Name:            "Protected Project",
		CreatedByUserID: owner.ID,
		IsDefault:       false,
	}
	db.DB.Create(&shareableProj)
	db.DB.Create(&models.ProjectMember{
		ProjectID: shareableProj.ID,
		UserID:    owner.ID,
		Role:      models.ProjectMemberRoleOwner,
		Status:    models.ProjectMemberStatusAccepted,
	})

	env := models.Environment{
		Name:      "Env 1",
		ProjectID: shareableProj.ID,
		UserID:    owner.ID,
	}
	db.DB.Create(&env)

	// Send invite (Status: PENDING)
	invitation := models.CollaborationInvitation{
		ProjectID: shareableProj.ID,
		InviterID: owner.ID,
		InviteeID: userB.ID,
		Role:      models.ProjectMemberRoleEditor,
		Status:    models.InvitePending,
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
	}
	db.DB.Create(&invitation)

	// User B attempts to access project
	reqProj, _ := http.NewRequest("GET", "/api/projects/"+shareableProj.ID, nil)
	reqProj.Header.Set("X-Test-User", userB.ID)
	wProj := httptest.NewRecorder()
	r.ServeHTTP(wProj, reqProj)
	if wProj.Code != http.StatusForbidden {
		t.Fatalf("Expected 403 Forbidden for pending invitee accessing project, got %d", wProj.Code)
	}

	// User B attempts to access environment files
	reqEnv, _ := http.NewRequest("GET", "/api/environments/"+env.ID+"/files", nil)
	reqEnv.Header.Set("X-Test-User", userB.ID)
	wEnv := httptest.NewRecorder()
	r.ServeHTTP(wEnv, reqEnv)
	if wEnv.Code != http.StatusNotFound && wEnv.Code != http.StatusForbidden {
		t.Fatalf("Expected 403/404 for pending invitee accessing environment files, got %d", wEnv.Code)
	}
}

func TestRejectInvitationGrantsNoAccess(t *testing.T) {
	setupTestDBForCollaboration(t)
	r := setupCollaborationRouter()

	owner := models.User{Email: "owner6@test.com", Username: "owner6"}
	userB := models.User{Email: "userb6@test.com", Username: "userb6"}
	db.DB.Create(&owner)
	db.DB.Create(&userB)

	shareableProj := models.Project{
		Name:            "Rejected Project",
		CreatedByUserID: owner.ID,
		IsDefault:       false,
	}
	db.DB.Create(&shareableProj)
	db.DB.Create(&models.ProjectMember{
		ProjectID: shareableProj.ID,
		UserID:    owner.ID,
		Role:      models.ProjectMemberRoleOwner,
		Status:    models.ProjectMemberStatusAccepted,
	})

	invitation := models.CollaborationInvitation{
		ProjectID: shareableProj.ID,
		InviterID: owner.ID,
		InviteeID: userB.ID,
		Role:      models.ProjectMemberRoleEditor,
		Status:    models.InvitePending,
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
	}
	db.DB.Create(&invitation)

	// User B rejects
	rejReq, _ := http.NewRequest("POST", "/api/invitations/"+invitation.ID+"/reject", nil)
	rejReq.Header.Set("X-Test-User", userB.ID)
	wRej := httptest.NewRecorder()
	r.ServeHTTP(wRej, rejReq)
	if wRej.Code != http.StatusOK {
		t.Fatalf("Expected 200 on rejection, got %d", wRej.Code)
	}

	// Ensure no ProjectMember was created
	var count int64
	db.DB.Model(&models.ProjectMember{}).Where("project_id = ? AND user_id = ?", shareableProj.ID, userB.ID).Count(&count)
	if count != 0 {
		t.Fatalf("Expected 0 project members for rejected user, got %d", count)
	}

	// Access must be denied
	accReq, _ := http.NewRequest("GET", "/api/projects/"+shareableProj.ID, nil)
	accReq.Header.Set("X-Test-User", userB.ID)
	wAcc := httptest.NewRecorder()
	r.ServeHTTP(wAcc, accReq)
	if wAcc.Code != http.StatusForbidden {
		t.Fatalf("Expected 403 Forbidden for rejected user, got %d", wAcc.Code)
	}
}

func TestRevokedInvitationCannotBeAccepted(t *testing.T) {
	setupTestDBForCollaboration(t)
	r := setupCollaborationRouter()

	owner := models.User{Email: "owner7@test.com", Username: "owner7"}
	userB := models.User{Email: "userb7@test.com", Username: "userb7"}
	db.DB.Create(&owner)
	db.DB.Create(&userB)

	shareableProj := models.Project{
		Name:            "Revocation Project",
		CreatedByUserID: owner.ID,
		IsDefault:       false,
	}
	db.DB.Create(&shareableProj)
	db.DB.Create(&models.ProjectMember{
		ProjectID: shareableProj.ID,
		UserID:    owner.ID,
		Role:      models.ProjectMemberRoleOwner,
		Status:    models.ProjectMemberStatusAccepted,
	})

	invitation := models.CollaborationInvitation{
		ProjectID: shareableProj.ID,
		InviterID: owner.ID,
		InviteeID: userB.ID,
		Role:      models.ProjectMemberRoleEditor,
		Status:    models.InvitePending,
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
	}
	db.DB.Create(&invitation)

	// Owner revokes invitation
	revReq, _ := http.NewRequest("DELETE", "/api/projects/"+shareableProj.ID+"/invitations/"+invitation.ID, nil)
	revReq.Header.Set("X-Test-User", owner.ID)
	wRev := httptest.NewRecorder()
	r.ServeHTTP(wRev, revReq)
	if wRev.Code != http.StatusOK {
		t.Fatalf("Expected 200 on revoke, got %d", wRev.Code)
	}

	// User B attempts to accept
	accReq, _ := http.NewRequest("POST", "/api/invitations/"+invitation.ID+"/accept", nil)
	accReq.Header.Set("X-Test-User", userB.ID)
	wAcc := httptest.NewRecorder()
	r.ServeHTTP(wAcc, accReq)
	if wAcc.Code != http.StatusBadRequest {
		t.Fatalf("Expected 400 Bad Request on revoked invitation acceptance, got %d", wAcc.Code)
	}
}

func TestRBACPermissionsViewerVsEditorVsOwner(t *testing.T) {
	setupTestDBForCollaboration(t)
	r := setupCollaborationRouter()

	owner := models.User{Email: "owner8@test.com", Username: "owner8"}
	editor := models.User{Email: "editor8@test.com", Username: "editor8"}
	viewer := models.User{Email: "viewer8@test.com", Username: "viewer8"}
	db.DB.Create(&owner)
	db.DB.Create(&editor)
	db.DB.Create(&viewer)

	proj := models.Project{
		Name:            "RBAC Test Project",
		CreatedByUserID: owner.ID,
		IsDefault:       false,
	}
	db.DB.Create(&proj)

	db.DB.Create(&models.ProjectMember{ProjectID: proj.ID, UserID: owner.ID, Role: models.ProjectMemberRoleOwner, Status: models.ProjectMemberStatusAccepted})
	db.DB.Create(&models.ProjectMember{ProjectID: proj.ID, UserID: editor.ID, Role: models.ProjectMemberRoleEditor, Status: models.ProjectMemberStatusAccepted})
	db.DB.Create(&models.ProjectMember{ProjectID: proj.ID, UserID: viewer.ID, Role: models.ProjectMemberRoleViewer, Status: models.ProjectMemberStatusAccepted})

	env := models.Environment{
		Name:      "RBAC Env",
		ProjectID: proj.ID,
		UserID:    owner.ID,
		Status:    models.StatusRunning,
	}
	db.DB.Create(&env)

	// 1. Viewer cannot restart environment (write operation - blocked by RBAC)
	reqRestartViewer, _ := http.NewRequest("POST", "/api/environments/"+env.ID+"/restart", nil)
	reqRestartViewer.Header.Set("X-Test-User", viewer.ID)
	wRestartViewer := httptest.NewRecorder()
	r.ServeHTTP(wRestartViewer, reqRestartViewer)
	if wRestartViewer.Code != http.StatusForbidden {
		t.Fatalf("Expected 403 Forbidden for Viewer restart, got %d: %s", wRestartViewer.Code, wRestartViewer.Body.String())
	}

	// 2. Viewer cannot update file content (write operation - blocked by RBAC)
	reqFileViewer, _ := http.NewRequest("POST", "/api/environments/"+env.ID+"/files/content", bytes.NewBufferString(`{"path":"main.go","content":"test"}`))
	reqFileViewer.Header.Set("Content-Type", "application/json")
	reqFileViewer.Header.Set("X-Test-User", viewer.ID)
	wFileViewer := httptest.NewRecorder()
	r.ServeHTTP(wFileViewer, reqFileViewer)
	if wFileViewer.Code != http.StatusForbidden {
		t.Fatalf("Expected 403 Forbidden for Viewer write file, got %d: %s", wFileViewer.Code, wFileViewer.Body.String())
	}

	// 3. Editor CAN write file (passes RBAC check, reaches workspace path)
	reqFileEditor, _ := http.NewRequest("POST", "/api/environments/"+env.ID+"/files/content", bytes.NewBufferString(`{"path":"main.go","content":"test"}`))
	reqFileEditor.Header.Set("Content-Type", "application/json")
	reqFileEditor.Header.Set("X-Test-User", editor.ID)
	wFileEditor := httptest.NewRecorder()
	r.ServeHTTP(wFileEditor, reqFileEditor)
	// Must NOT be 403 Forbidden (authorization passed!)
	if wFileEditor.Code == http.StatusForbidden {
		t.Fatalf("Editor should not be forbidden from writing files, got %d", wFileEditor.Code)
	}

	// 4. Editor CANNOT manage members (invite user)
	payload := `{"identifier":"someone@test.com","role":"VIEWER"}`
	reqInviteEditor, _ := http.NewRequest("POST", "/api/projects/"+proj.ID+"/invite", bytes.NewBufferString(payload))
	reqInviteEditor.Header.Set("Content-Type", "application/json")
	reqInviteEditor.Header.Set("X-Test-User", editor.ID)
	wInviteEditor := httptest.NewRecorder()
	r.ServeHTTP(wInviteEditor, reqInviteEditor)
	if wInviteEditor.Code != http.StatusForbidden {
		t.Fatalf("Expected 403 Forbidden for Editor inviting member, got %d", wInviteEditor.Code)
	}

	// 5. Editor CANNOT delete environment
	reqDelEditor, _ := http.NewRequest("DELETE", "/api/environments/"+env.ID, nil)
	reqDelEditor.Header.Set("X-Test-User", editor.ID)
	wDelEditor := httptest.NewRecorder()
	r.ServeHTTP(wDelEditor, reqDelEditor)
	if wDelEditor.Code != http.StatusForbidden {
		t.Fatalf("Expected 403 Forbidden for Editor deleting environment, got %d", wDelEditor.Code)
	}
}

func TestOwnerCannotRemoveThemselves(t *testing.T) {
	setupTestDBForCollaboration(t)
	r := setupCollaborationRouter()

	owner := models.User{Email: "owner9@test.com", Username: "owner9"}
	db.DB.Create(&owner)

	proj := models.Project{
		Name:            "Owner Removal Test",
		CreatedByUserID: owner.ID,
		IsDefault:       false,
	}
	db.DB.Create(&proj)
	db.DB.Create(&models.ProjectMember{
		ProjectID: proj.ID,
		UserID:    owner.ID,
		Role:      models.ProjectMemberRoleOwner,
		Status:    models.ProjectMemberStatusAccepted,
	})

	delReq, _ := http.NewRequest("DELETE", "/api/projects/"+proj.ID+"/collaborators/"+owner.ID, nil)
	delReq.Header.Set("X-Test-User", owner.ID)
	wDel := httptest.NewRecorder()
	r.ServeHTTP(wDel, delReq)

	if wDel.Code != http.StatusBadRequest {
		t.Fatalf("Expected 400 Bad Request when owner attempts to remove themselves, got %d: %s", wDel.Code, wDel.Body.String())
	}
}

func TestRemovedMemberImmediatelyLosesAccess(t *testing.T) {
	setupTestDBForCollaboration(t)
	r := setupCollaborationRouter()

	owner := models.User{Email: "owner10@test.com", Username: "owner10"}
	editor := models.User{Email: "editor10@test.com", Username: "editor10"}
	db.DB.Create(&owner)
	db.DB.Create(&editor)

	proj := models.Project{
		Name:            "Removal Revocation Project",
		CreatedByUserID: owner.ID,
		IsDefault:       false,
	}
	db.DB.Create(&proj)
	db.DB.Create(&models.ProjectMember{ProjectID: proj.ID, UserID: owner.ID, Role: models.ProjectMemberRoleOwner, Status: models.ProjectMemberStatusAccepted})
	db.DB.Create(&models.ProjectMember{ProjectID: proj.ID, UserID: editor.ID, Role: models.ProjectMemberRoleEditor, Status: models.ProjectMemberStatusAccepted})

	env := models.Environment{
		Name:      "Env Removal",
		ProjectID: proj.ID,
		UserID:    owner.ID,
		Status:    models.StatusRunning,
	}
	db.DB.Create(&env)

	// Editor can access initially
	req1, _ := http.NewRequest("GET", "/api/projects/"+proj.ID, nil)
	req1.Header.Set("X-Test-User", editor.ID)
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)
	if w1.Code != http.StatusOK {
		t.Fatalf("Expected 200 before removal, got %d", w1.Code)
	}

	// Owner removes editor
	remReq, _ := http.NewRequest("DELETE", "/api/projects/"+proj.ID+"/collaborators/"+editor.ID, nil)
	remReq.Header.Set("X-Test-User", owner.ID)
	wRem := httptest.NewRecorder()
	r.ServeHTTP(wRem, remReq)
	if wRem.Code != http.StatusOK {
		t.Fatalf("Expected 200 on member removal, got %d", wRem.Code)
	}

	// Editor immediately denied project access
	req2, _ := http.NewRequest("GET", "/api/projects/"+proj.ID, nil)
	req2.Header.Set("X-Test-User", editor.ID)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusForbidden {
		t.Fatalf("Expected 403 Forbidden after removal, got %d", w2.Code)
	}

	// Editor immediately denied environment restart
	reqRestart, _ := http.NewRequest("POST", "/api/environments/"+env.ID+"/restart", nil)
	reqRestart.Header.Set("X-Test-User", editor.ID)
	wRestart := httptest.NewRecorder()
	r.ServeHTTP(wRestart, reqRestart)
	if wRestart.Code != http.StatusForbidden {
		t.Fatalf("Expected 403 Forbidden for restart after removal, got %d", wRestart.Code)
	}
}

func TestCrossProjectAndPersonalContainerAccessBlocked(t *testing.T) {
	setupTestDBForCollaboration(t)
	r := setupCollaborationRouter()

	userA := models.User{Email: "usera@test.com", Username: "usera"}
	userB := models.User{Email: "userb11@test.com", Username: "userb11"}
	db.DB.Create(&userA)
	db.DB.Create(&userB)

	// User A's Default Project
	defaultA := models.Project{
		Name:            "Personal Space User A",
		CreatedByUserID: userA.ID,
		IsDefault:       true,
	}
	db.DB.Create(&defaultA)
	db.DB.Create(&models.ProjectMember{ProjectID: defaultA.ID, UserID: userA.ID, Role: models.ProjectMemberRoleOwner, Status: models.ProjectMemberStatusAccepted})

	personalEnv := models.Environment{
		Name:      "User A Personal Env",
		ProjectID: defaultA.ID,
		UserID:    userA.ID,
		Status:    models.StatusRunning,
	}
	db.DB.Create(&personalEnv)

	// User B attempts to restart User A's personal environment
	reqRestart, _ := http.NewRequest("POST", "/api/environments/"+personalEnv.ID+"/restart", nil)
	reqRestart.Header.Set("X-Test-User", userB.ID)
	wRestart := httptest.NewRecorder()
	r.ServeHTTP(wRestart, reqRestart)

	if wRestart.Code != http.StatusForbidden {
		t.Fatalf("Expected 403 Forbidden when User B accesses User A's personal environment, got %d: %s", wRestart.Code, wRestart.Body.String())
	}
}

func TestConcurrentInvitationAcceptance(t *testing.T) {
	setupTestDBForCollaboration(t)
	r := setupCollaborationRouter()

	owner := models.User{Email: "owner12@test.com", Username: "owner12"}
	userB := models.User{Email: "userb12@test.com", Username: "userb12"}
	db.DB.Create(&owner)
	db.DB.Create(&userB)

	proj := models.Project{
		Name:            "Concurrent Acceptance Project",
		CreatedByUserID: owner.ID,
		IsDefault:       false,
	}
	db.DB.Create(&proj)
	db.DB.Create(&models.ProjectMember{ProjectID: proj.ID, UserID: owner.ID, Role: models.ProjectMemberRoleOwner, Status: models.ProjectMemberStatusAccepted})

	invitation := models.CollaborationInvitation{
		ProjectID: proj.ID,
		InviterID: owner.ID,
		InviteeID: userB.ID,
		Role:      models.ProjectMemberRoleEditor,
		Status:    models.InvitePending,
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
	}
	db.DB.Create(&invitation)

	// Run concurrent acceptance requests
	var wg sync.WaitGroup
	var codes [5]int
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			req, _ := http.NewRequest("POST", "/api/invitations/"+invitation.ID+"/accept", nil)
			req.Header.Set("X-Test-User", userB.ID)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			codes[idx] = w.Code
		}(i)
	}
	wg.Wait()

	// Verify only 1 ProjectMember record exists for User B in proj
	var memberCount int64
	db.DB.Model(&models.ProjectMember{}).Where("project_id = ? AND user_id = ?", proj.ID, userB.ID).Count(&memberCount)
	if memberCount != 1 {
		t.Fatalf("Expected exactly 1 membership created after concurrent acceptance, got %d", memberCount)
	}
}
