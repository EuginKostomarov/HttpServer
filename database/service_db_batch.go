package database

import (
	"fmt"
	"database/sql"
)

// GetProjectsWithClients получает проекты с информацией о клиентах одним запросом
func (db *ServiceDB) GetProjectsWithClients(projectIDs []int) (map[int]*ClientProjectWithClient, error) {
	if len(projectIDs) == 0 {
		return make(map[int]*ClientProjectWithClient), nil
	}

	// Создаем placeholders для IN запроса
	placeholders := make([]string, len(projectIDs))
	args := make([]interface{}, len(projectIDs))
	for i, id := range projectIDs {
		placeholders[i] = "?"
		args[i] = id
	}

	// Правильный способ создания placeholders
	placeholdersStr := ""
	for i := 0; i < len(projectIDs); i++ {
		if i > 0 {
			placeholdersStr += ","
		}
		placeholdersStr += "?"
	}

	query := fmt.Sprintf(`
		SELECT 
			cp.id, cp.client_id, cp.name, cp.project_type, cp.description, cp.source_system,
			cp.status, cp.target_quality_score, cp.created_at, cp.updated_at,
			c.id, c.name, c.legal_name, c.description, c.contact_email, c.contact_phone,
			c.tax_id, c.status, c.created_by, c.created_at, c.updated_at
		FROM client_projects cp
		INNER JOIN clients c ON cp.client_id = c.id
		WHERE cp.id IN (%s)
	`, placeholdersStr)

	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query projects with clients: %w", err)
	}
	defer rows.Close()

	result := make(map[int]*ClientProjectWithClient)
	for rows.Next() {
		var project ClientProjectWithClient
		var client Client

		err := rows.Scan(
			&project.ID, &project.ClientID, &project.Name, &project.ProjectType,
			&project.Description, &project.SourceSystem, &project.Status,
			&project.TargetQualityScore, &project.CreatedAt, &project.UpdatedAt,
			&client.ID, &client.Name, &client.LegalName, &client.Description,
			&client.ContactEmail, &client.ContactPhone, &client.TaxID,
			&client.Status, &client.CreatedBy, &client.CreatedAt, &client.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan project with client: %w", err)
		}

		project.Client = &client
		result[project.ID] = &project
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating projects: %w", err)
	}

	return result, nil
}

// ClientProjectWithClient содержит проект с информацией о клиенте
type ClientProjectWithClient struct {
	ClientProject
	Client *Client
}

// GetProjectDatabaseWithClient получает базу данных проекта с информацией о проекте и клиенте
func (db *ServiceDB) GetProjectDatabaseWithClient(dbID int) (*ProjectDatabaseWithProject, error) {
	query := `
		SELECT 
			pd.id, pd.client_project_id, pd.name, pd.file_path, pd.description,
			pd.is_active, pd.file_size, pd.last_used_at, pd.created_at, pd.updated_at,
			cp.id, cp.client_id, cp.name, cp.project_type, cp.description, cp.source_system,
			cp.status, cp.target_quality_score, cp.created_at, cp.updated_at,
			c.id, c.name, c.legal_name, c.description, c.contact_email, c.contact_phone,
			c.tax_id, c.status, c.created_by, c.created_at, c.updated_at
		FROM project_databases pd
		INNER JOIN client_projects cp ON pd.client_project_id = cp.id
		INNER JOIN clients c ON cp.client_id = c.id
		WHERE pd.id = ?
	`

	var dbInfo ProjectDatabaseWithProject
	var project ClientProjectWithClient
	var client Client

	err := db.conn.QueryRow(query, dbID).Scan(
		&dbInfo.ID, &dbInfo.ClientProjectID, &dbInfo.Name, &dbInfo.FilePath,
		&dbInfo.Description, &dbInfo.IsActive, &dbInfo.FileSize, &dbInfo.LastUsedAt,
		&dbInfo.CreatedAt, &dbInfo.UpdatedAt,
		&project.ID, &project.ClientID, &project.Name, &project.ProjectType,
		&project.Description, &project.SourceSystem, &project.Status,
		&project.TargetQualityScore, &project.CreatedAt, &project.UpdatedAt,
		&client.ID, &client.Name, &client.LegalName, &client.Description,
		&client.ContactEmail, &client.ContactPhone, &client.TaxID,
		&client.Status, &client.CreatedBy, &client.CreatedAt, &client.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("database not found")
		}
		return nil, fmt.Errorf("failed to get database with project and client: %w", err)
	}

	project.Client = &client
	dbInfo.Project = &project

	return &dbInfo, nil
}

// ProjectDatabaseWithProject содержит базу данных проекта с информацией о проекте и клиенте
type ProjectDatabaseWithProject struct {
	ProjectDatabase
	Project *ClientProjectWithClient
}

