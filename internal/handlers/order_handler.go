package handlers

import (
	"net/http"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/gin-contrib/sessions"

	"github.com/Keoroanthony/go-ecommerce/internal/db"
    "github.com/Keoroanthony/go-ecommerce/internal/models"
	"github.com/Keoroanthony/go-ecommerce/internal/notifier"
)

type CreateOrderRequest struct {
    ProductIDs []uint `json:"product_ids"`
}

func CreateOrder (c *gin.Context) {

	sess := sessions.Default(c)
	custID, ok := sess.Get("customer_id").(uint)

	if !ok || custID == 0 {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req CreateOrderRequest

    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
        return
    }

	if len(req.ProductIDs) == 0 {
        c.JSON(http.StatusBadRequest, gin.H{"error": "product_ids required"})
        return
    }

	var customer models.Customer
    if err := db.DB.First(&customer, custID).Error; err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "customer not found"})
        return
    }

	tx := db.DB.Begin()

	if tx.Error != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to start transaction"})
        return
    }

	order := models.Order{

        CustomerID: customer.ID,
    }

	if err := tx.Create(&order).Error; err != nil {
		
		tx.Rollback()

        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    var orderItems []models.OrderItem
	var totalOrderPrice float64

	for _, productID := range req.ProductIDs {

		var product models.Product

		if err := tx.First(&product, productID).Error; err != nil {

			tx.Rollback()
		
			errorMessage := fmt.Sprintf("Product not found with ID: %d", productID)
			c.JSON(http.StatusNotFound, gin.H{"error": errorMessage})
			return
		}

		orderItem := models.OrderItem{
			OrderID:   order.ID, 
			ProductID: product.ID,
			Quantity:  1,
			Price:     product.Price,
		}

		orderItems = append(orderItems, orderItem)
		totalOrderPrice += product.Price
	}

	if len(orderItems) > 0 {
		if err := tx.CreateInBatches(&orderItems, len(orderItems)).Error; err != nil { 

			tx.Rollback()

			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create order items"})
			return
		}
	}
	
	if err := tx.Preload("Items").First(&order, order.ID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve order with items"})
		return
	}

	tx.Commit()

	go func(customer models.Customer, order models.Order, totalOrderPrice float64) {

		if err := notifier.SendSMS(customer.Phone, order.ID, totalOrderPrice); err != nil {
			fmt.Printf("Failed to send SMS for order %d to %s: %v\n", order.ID, customer.Phone, err)
		}
	}(customer, order, totalOrderPrice)

	go func(customer models.Customer, order models.Order, totalOrderPrice float64) {

		if err := notifier.SendEmail(customer.Email, customer.Name, order.ID, totalOrderPrice); err != nil {
			fmt.Printf("Failed to send SMS for order %d to %s: %v\n", order.ID, customer.Phone, err)
		}
	}(customer, order, totalOrderPrice)

	c.JSON(http.StatusCreated, gin.H{"message": "order created successfully", "order": order})


}