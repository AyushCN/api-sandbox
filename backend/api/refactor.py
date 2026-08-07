import re
import sys

def process_file(filepath):
    with open(filepath, 'r') as f:
        content = f.read()

    # Add helpers
    helpers = """
func applyEnvironmentScope(query *gorm.DB, userID interface{}) *gorm.DB {
	projectIDs := getUserProjectIDs(userID)
	if len(projectIDs) > 0 {
		return query.Where("project_id IN ? OR user_id = ?", projectIDs, userID)
	}
	return query.Where("user_id = ?", userID)
}

func executeWithRetry(maxAttempts int, initialBackoff time.Duration, fn func() error) error {
	var err error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		err = fn()
		if err == nil {
			return nil
		}
		if attempt < maxAttempts-1 {
			time.Sleep(initialBackoff * time.Duration(math.Pow(2, float64(attempt))))
		}
	}
	return err
}

func GetEnvironments(c *gin.Context) {"""
    content = content.replace("func GetEnvironments(c *gin.Context) {", helpers)

    # Replace GetEnvironments logic
    old_env_logic = """	// Get User's Projects
	var projectCollabs []models.ProjectCollaborator
	db.DB.Where("user_id = ?", userID).Find(&projectCollabs)
	var projectIDs []string
	for _, pc := range projectCollabs {
		projectIDs = append(projectIDs, pc.ProjectID)
	}

	for attempt := 0; attempt < 3; attempt++ {
		query := db.DB.WithContext(context.Background())
		
		conditions := []string{"user_id = ?"}
		args := []interface{}{userID}

		if len(projectIDs) > 0 {
			conditions = append(conditions, "project_id IN ?")
			args = append(args, projectIDs)
		}

		query = query.Where(strings.Join(conditions, " OR "), args...)
		err = query.Order("created_at desc").
			Offset(offset).
			Limit(params.Limit).
			Find(&environments).Error
			
		if err == nil {
			break
		}
		if attempt < 2 {
			time.Sleep(100 * time.Millisecond * time.Duration(math.Pow(2, float64(attempt))))
		}
	}"""
    
    new_env_logic = """	err = executeWithRetry(3, 100*time.Millisecond, func() error {
		query := applyEnvironmentScope(db.DB.WithContext(context.Background()), userID)
		return query.Order("created_at desc").
			Offset(offset).
			Limit(params.Limit).
			Find(&environments).Error
	})"""
    content = content.replace(old_env_logic, new_env_logic)

    # Replace GetEnvironment logic
    old_get_env = """	// Get User's Projects
	var projectCollabs []models.ProjectCollaborator
	db.DB.Where("user_id = ?", userID).Find(&projectCollabs)
	var projectIDs []string
	for _, c := range projectCollabs {
		projectIDs = append(projectIDs, c.ProjectID)
	}

	var err error
	for attempt := 0; attempt < 3; attempt++ {
		query := db.DB.WithContext(context.Background()).
			Preload("Logs", func(db *gorm.DB) *gorm.DB {
				return db.Order("timestamp desc").Limit(100)
			}).
			Preload("Metrics", func(db *gorm.DB) *gorm.DB {
				return db.Order("timestamp desc").Limit(100)
			})

		conditions := []string{"user_id = ?"}
		args := []interface{}{userID}

		if len(projectIDs) > 0 {
			conditions = append(conditions, "project_id IN ?")
			args = append(args, projectIDs)
		}

		query = query.Where(strings.Join(conditions, " OR "), args...)

		err = query.First(&env, "id = ?", id).Error
		
		if err == nil {
			break
		}
		if attempt < 2 {
			time.Sleep(100 * time.Millisecond * time.Duration(math.Pow(2, float64(attempt))))
		}
	}"""

    new_get_env = """	err := executeWithRetry(3, 100*time.Millisecond, func() error {
		query := applyEnvironmentScope(db.DB.WithContext(context.Background()), userID).
			Preload("Logs", func(db *gorm.DB) *gorm.DB {
				return db.Order("timestamp desc").Limit(100)
			}).
			Preload("Metrics", func(db *gorm.DB) *gorm.DB {
				return db.Order("timestamp desc").Limit(100)
			})

		return query.First(&env, "id = ?", id).Error
	})"""
    content = content.replace(old_get_env, new_get_env)

    # Replace StreamLogs logic
    old_stream = """	projectIDs := getUserProjectIDs(userID)

	// Verify access
	var envCount int64
	query := db.DB.Model(&models.Environment{}).Where("id = ?", envID)
	if len(projectIDs) > 0 {
		query = query.Where("project_id IN ? OR user_id = ?", projectIDs, userID)
	} else {
		query = query.Where("user_id = ?", userID)
	}"""
    
    new_stream = """	// Verify access
	var envCount int64
	query := applyEnvironmentScope(db.DB.Model(&models.Environment{}), userID).Where("id = ?", envID)"""
    content = content.replace(old_stream, new_stream)

    # Replace common pattern
    common_pattern = """	projectIDs := getUserProjectIDs(userID)

	var env models.Environment
	query := db.DB
	if len(projectIDs) > 0 {
		query = query.Where("project_id IN ? OR user_id = ?", projectIDs, userID)
	} else {
		query = query.Where("user_id = ?", userID)
	}"""

    new_common = """	var env models.Environment
	query := applyEnvironmentScope(db.DB, userID)"""
    content = content.replace(common_pattern, new_common)

    with open(filepath, 'w') as f:
        f.write(content)
    
    print("Done refactoring")

if __name__ == "__main__":
    process_file("/home/swordrookie/projects/api_sandbox_links/api-sandbox/backend/api/handlers.go")
