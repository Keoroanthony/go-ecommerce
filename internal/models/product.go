package models

type Product struct {
	
    ID         uint     `gorm:"primaryKey"`
    Name       string   `gorm:"not null"`
    Price      float64  `gorm:"not null"`
    CategoryID uint     `gorm:"index;not null"`
    Category   Category
}
