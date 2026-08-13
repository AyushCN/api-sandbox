package main

import (
	"fmt"
	"github.com/api-sandbox/backend/db"
	"github.com/api-sandbox/backend/models"
)

func main() {
	db.InitDB()
	var deps []models.Deployment
	db.DB.Find(&deps)
	for _, d := range deps {
		fmt.Printf("ID: %s, Name: %s, Status: %s\n", d.ID, d.Name, d.Status)
	}
}
