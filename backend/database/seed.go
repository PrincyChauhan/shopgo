package database

import (
	"errors"
	"log"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"shopgo/models"
)

// SeedAdmin creates the first admin account if it does not exist yet.
// There is no public way to become admin, so we need this bootstrap.
func SeedAdmin(db *gorm.DB, email, password string) {
	var u models.User
	err := db.Where("email = ?", email).First(&u).Error
	if err == nil {
		return // already exists
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		log.Fatalf("seed check failed: %v", err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("seed hash failed: %v", err)
	}
	admin := models.User{Name: "Admin", Email: email, Password: string(hash), Role: models.RoleAdmin}
	if err := db.Create(&admin).Error; err != nil {
		log.Fatalf("seed admin failed: %v", err)
	}
	log.Printf("Seeded admin account: %s", email)
}

// SeedCatalog adds sample categories and products, but only into an empty store.
func SeedCatalog(db *gorm.DB) {
	var count int64
	db.Model(&models.Product{}).Count(&count)
	if count > 0 {
		return
	}

	names := []string{"Electronics", "Books", "Clothing", "Home & Kitchen"}
	cats := map[string]models.Category{}
	for _, n := range names {
		c := models.Category{Name: n}
		if err := db.Where("name = ?", n).FirstOrCreate(&c).Error; err != nil {
			log.Fatalf("seed category failed: %v", err)
		}
		cats[n] = c
	}

	// Prices are in cents.
	products := []models.Product{
		{Name: "Wireless Headphones", Description: "Over-ear Bluetooth headphones with 30-hour battery life.", Price: 449900, Stock: 25, CategoryID: cats["Electronics"].ID},
		{Name: "Mechanical Keyboard", Description: "Compact 75% layout keyboard with tactile switches.", Price: 389900, Stock: 15, CategoryID: cats["Electronics"].ID},
		{Name: "USB-C Fast Charger 65W", Description: "Charges laptops, tablets and phones from one small brick.", Price: 129900, Stock: 60, CategoryID: cats["Electronics"].ID},
		{Name: "The Go Programming Language", Description: "A clear, thorough guide to writing Go programs.", Price: 69900, Stock: 40, CategoryID: cats["Books"].ID},
		{Name: "Atomic Habits", Description: "Practical ways to build good habits and break bad ones.", Price: 39900, Stock: 80, CategoryID: cats["Books"].ID},
		{Name: "Cotton T-Shirt", Description: "Soft everyday t-shirt, 100% combed cotton.", Price: 59900, Stock: 100, CategoryID: cats["Clothing"].ID},
		{Name: "Denim Jacket", Description: "Classic fit jacket with a durable stonewashed finish.", Price: 179900, Stock: 12, CategoryID: cats["Clothing"].ID},
		{Name: "Stainless Steel Bottle", Description: "Keeps drinks cold for 24 hours or hot for 12. 750ml.", Price: 79900, Stock: 50, CategoryID: cats["Home & Kitchen"].ID},
		{Name: "Non-stick Frying Pan", Description: "26cm pan with an induction-compatible base.", Price: 99900, Stock: 0, CategoryID: cats["Home & Kitchen"].ID},
	}
	if err := db.Create(&products).Error; err != nil {
		log.Fatalf("seed products failed: %v", err)
	}
	log.Printf("Seeded %d sample products", len(products))
}
