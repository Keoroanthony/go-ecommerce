package models

type Category struct {

	ID       uint       `gorm:"primaryKey"`
    Name     string     `gorm:"uniqueIndex;not null"`
    ParentID *uint      `gorm:"index"` // nullable
    Parent   *Category
    Children []Category `gorm:"foreignKey:ParentID"`
}