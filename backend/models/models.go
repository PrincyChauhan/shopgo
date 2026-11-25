package models

import "time"

// Base gives every table an id and timestamps, with clean JSON names.
type Base struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"-"`
}

type Role string

const (
	RoleCustomer Role = "customer"
	RoleAdmin    Role = "admin"
)

type User struct {
	Base
	Name     string `gorm:"not null" json:"name"`
	Email    string `gorm:"uniqueIndex;not null" json:"email"`
	Password string `gorm:"not null" json:"-"` // json:"-" means it is NEVER sent to clients
	Role     Role   `gorm:"not null;default:customer" json:"role"`
}

type Category struct {
	Base
	Name string `gorm:"uniqueIndex;not null" json:"name"`
}

// Price is stored in cents (int64) to avoid floating-point rounding bugs with money.
type Product struct {
	Base
	Name        string   `gorm:"not null;index" json:"name"`
	Description string   `json:"description"`
	Price       int64    `gorm:"not null" json:"price"`
	Stock       int      `gorm:"not null;default:0" json:"stock"`
	ImageURL    string   `json:"image_url"`
	CategoryID  uint     `gorm:"index" json:"category_id"`
	Category    Category `json:"category"`
}

// CartItem: one row per (user, product). The composite unique index
// stops the same product appearing twice in one user's cart.
type CartItem struct {
	Base
	UserID    uint    `gorm:"not null;uniqueIndex:idx_user_product" json:"-"`
	ProductID uint    `gorm:"not null;uniqueIndex:idx_user_product" json:"product_id"`
	Quantity  int     `gorm:"not null" json:"quantity"`
	Product   Product `json:"product"`
}

type OrderStatus string

const (
	StatusPending   OrderStatus = "pending"
	StatusPaid      OrderStatus = "paid"
	StatusShipped   OrderStatus = "shipped"
	StatusDelivered OrderStatus = "delivered"
	StatusCancelled OrderStatus = "cancelled"
)

type Order struct {
	Base
	UserID uint        `gorm:"not null;index" json:"user_id"`
	Total  int64       `gorm:"not null" json:"total"`
	Status OrderStatus `gorm:"not null;default:pending" json:"status"`

	// Where to deliver (copied onto the order so later profile changes don't alter old orders)
	ShipName    string `gorm:"not null;default:''" json:"ship_name"`
	ShipPhone   string `gorm:"not null;default:''" json:"ship_phone"`
	ShipAddress string `gorm:"not null;default:''" json:"ship_address"`
	ShipCity    string `gorm:"not null;default:''" json:"ship_city"`
	ShipState   string `gorm:"not null;default:''" json:"ship_state"`
	ShipPincode string `gorm:"not null;default:''" json:"ship_pincode"`

	Items []OrderItem `json:"items"`
}

// OrderItem stores a SNAPSHOT of name and price at purchase time,
// so old orders stay correct even if the product changes later.
type OrderItem struct {
	Base
	OrderID   uint   `gorm:"not null;index" json:"-"`
	ProductID uint   `gorm:"not null" json:"product_id"`
	Name      string `gorm:"not null" json:"name"`
	Price     int64  `gorm:"not null" json:"price"`
	Quantity  int    `gorm:"not null" json:"quantity"`
}

// Payment records each attempt to pay for an order through a gateway.
// Amount is in paise, same as every other price in the app.
type Payment struct {
	Base
	OrderID          uint   `gorm:"not null;index" json:"order_id"`
	Provider         string `gorm:"not null" json:"provider"`
	GatewayOrderID   string `gorm:"uniqueIndex;not null" json:"gateway_order_id"`
	GatewayPaymentID string `gorm:"not null;default:''" json:"gateway_payment_id"`
	Amount           int64  `gorm:"not null" json:"amount"`
	Status           string `gorm:"not null;default:created" json:"status"` // created | paid
}
