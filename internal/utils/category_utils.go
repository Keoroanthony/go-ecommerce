package utils

import (
    "github.com/Keoroanthony/go-ecommerce/internal/db"
    "github.com/Keoroanthony/go-ecommerce/internal/models"
)

func GetAllCategoryIDs(rootID uint) ([]uint, error) {
    var result []uint
    result = append(result, rootID)

    var queue = []uint{rootID}

    for len(queue) > 0 {
        current := queue[0]
        queue = queue[1:]

        var children []models.Category
        err := db.DB.Where("parent_id = ?", current).Find(&children).Error
        if err != nil {
            return nil, err
        }

        for _, child := range children {
            result = append(result, child.ID)
            queue = append(queue, child.ID)
        }
    }

    return result, nil
}
