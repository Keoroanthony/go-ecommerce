package handlers

import (
	"net/http"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/Keoroanthony/go-ecommerce/internal/db"
	"github.com/Keoroanthony/go-ecommerce/internal/models"
)

type CreateCategoryRequest struct {
	Name     string `json:"name" binding:"required"`
	ParentID *uint  `json:"parent_id"`
}


func CreateCategory(c *gin.Context) {
	var req CreateCategoryRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.ParentID != nil {
		var parentCategory models.Category
	
		if err := db.DB.First(&parentCategory, *req.ParentID).Error; err != nil {

			errorMessage := fmt.Sprintf("Parent category not found with ID: %d", *req.ParentID)
	
			c.JSON(http.StatusNotFound, gin.H{"error": errorMessage})
			return
		}
	}

	category := models.Category{
		Name:     req.Name,
		ParentID: req.ParentID,
	}


	if err := db.DB.Create(&category).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := db.DB.Preload("Parent").First(&category, category.ID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve category with parent details"})
		return
	}

	c.JSON(http.StatusCreated, category)
}