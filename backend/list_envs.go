package main

import (
	"fmt"
	"github.com/api-sandbox/backend/db"
	"github.com/api-sandbox/backend/models"
)

func main() {
	db.InitDB()
	var envs []models.Environment
	db.DB.Find(&envs)
	for _, e := range envs {
		fmt.Printf("ID: %s, Name: %s, Status: %s\n", e.ID, e.Name, e.Status)
	}
}
