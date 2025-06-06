package handlers

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/Keoroanthony/go-ecommerce/internal/db"
	"github.com/Keoroanthony/go-ecommerce/internal/models"
	"github.com/Keoroanthony/go-ecommerce/internal/utils" // Ensure utils is imported
	"gorm.io/gorm"
)

type CreateProductRequest struct {
	Name       string  `json:"name" binding:"required"`
	Price      float64 `json:"price" binding:"required,gt=0"`
	CategoryID uint    `json:"category_id" binding:"required"`
}

func CreateProduct(c *gin.Context) {
	var req CreateProductRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var category models.Category
	if err := db.DB.First(&category, req.CategoryID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			errorMessage := fmt.Sprintf("Category not found with ID: %d", req.CategoryID)
			c.JSON(http.StatusNotFound, gin.H{"error": errorMessage})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error checking category existence"})
		}
		return
	}

	product := models.Product{
		Name:       req.Name,
		Price:      req.Price,
		CategoryID: req.CategoryID,
	}

	if err := db.DB.Create(&product).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := db.DB.Preload("Category").First(&product, product.ID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve Product with Category details"})
		return
	}

	c.JSON(http.StatusCreated, product)
}

func GetAveragePrice(c *gin.Context) {
	categoryIDParam := c.Query("category_id")
	if categoryIDParam == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "category_id is required"})
		return
	}

	var categoryID uint
	if _, err := fmt.Sscan(categoryIDParam, &categoryID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid category_id"})
		return
	}


	categoryIDs, err := utils.GetAllCategoryIDs(categoryID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var avg float64
	err = db.DB.
		Model(&models.Product{}).
		Where("category_id IN ?", categoryIDs).
		Select("COALESCE(AVG(price), 0)").
		Scan(&avg).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"category_id": categoryID, "average_price": avg})
}