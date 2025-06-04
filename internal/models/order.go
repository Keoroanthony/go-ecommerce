package models

import "time"

type Order struct {
    ID         uint        `gorm:"primaryKey"`
    CustomerID uint        `gorm:"index;not null"`
    Customer   Customer
    CreatedAt  time.Time
    Items      []OrderItem `gorm:"foreignKey:OrderID"`
}

type OrderItem struct {
    ID        uint    `gorm:"primaryKey"`
    OrderID   uint    `gorm:"index;not null"`
    ProductID uint    `gorm:"index;not null"`
    Quantity  uint    `gorm:"not null"`
    Price     float64 `gorm:"not null"`
    Product   Product
    CreatedAt time.Time
}