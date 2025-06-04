package models

type Customer struct {

	ID       uint   `gorm:"primaryKey"`
    Name     string `gorm:"not null"`
    Email    string `gorm:"uniqueIndex;not null"`
    Phone    string `gorm:"not null"`
    OIDCID   string `gorm:"uniqueIndex"` // OpenID Connect identifier
}