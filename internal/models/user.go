package models

import "time"

type User struct {

	ID         uint       `gorm:"primaryKey"`
    Name       string     `gorm:"not null"`
    Email      string     `gorm:"uniqueIndex;not null"`
	Role       string     `gorm:"not null"`
	CreatedAt  time.Time
}