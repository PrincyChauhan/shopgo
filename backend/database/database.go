package database

import (
	"log"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"shopgo/config"
	"shopgo/models"
)

// Connect connects to PostgreSQL and runs AutoMigrate on all models.
func Connect(cfg *config.Config) *gorm.DB {
	log.Println("Connecting to PostgreSQL...")
	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{})
	if err != nil {
		log.Fatalf("cannot connect to database: %v", err)
	}

	// AutoMigrate creates missing tables and columns. Safe to run on every start.
	err = db.AutoMigrate(
		&models.User{},
		&models.Category{},
		&models.Product{},
		&models.CartItem{},
		&models.Order{},
		&models.OrderItem{},
		&models.Payment{},
	)
	if err != nil {
		log.Fatalf("migration failed: %v", err)
	}

	return db
}
