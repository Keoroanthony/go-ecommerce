package handlers_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Keoroanthony/go-ecommerce/internal/db"
	"github.com/Keoroanthony/go-ecommerce/internal/handlers"
	"github.com/Keoroanthony/go-ecommerce/internal/models"

)


func setupProductTestRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
	gin.SetMode(gin.TestMode)

	// Initialize an in-memory SQLite database
	testDB, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		panic("failed to connect test database: " + err.Error())
	}

	// Auto-migrate all relevant models
	err = testDB.AutoMigrate(&models.Product{}, &models.Category{})
	if err != nil {
		panic("failed to auto-migrate models: " + err.Error())
	}

	originalDB := db.DB
	db.SetTestDB(testDB) // Set the test database for the handlers

	r := gin.New()
	r.Use(gin.Recovery())

	api := r.Group("/api")
	{
		api.POST("/products", handlers.CreateProduct)
		api.GET("/products/average", handlers.GetAveragePrice)
	}

	t.Cleanup(func() {
		db.SetTestDB(originalDB) // Restore original DB after tests
	})

	return r, testDB
}

func createProductRequest(method, path string, body interface{}) *http.Request {
	var reqBody []byte
	if body != nil {
		reqBody, _ = json.Marshal(body)
	}
	req := httptest.NewRequest(method, path, bytes.NewBuffer(reqBody))
	req.Header.Set("Content-Type", "application/json")
	return req
}

// TestCreateProductHandler
func TestCreateProductHandler(t *testing.T) {
	router, testDB := setupProductTestRouter(t)

	// Seed a category for testing
	category := models.Category{Name: "Electronics"}
	testDB.Create(&category)

	t.Run("Successfully creates a product", func(t *testing.T) {
		reqBody := handlers.CreateProductRequest{
			Name:       "Laptop",
			Price:      1200.00,
			CategoryID: category.ID,
		}
		recorder := httptest.NewRecorder()
		req := createProductRequest(http.MethodPost, "/api/products", reqBody)
		router.ServeHTTP(recorder, req)

		assert.Equal(t, http.StatusCreated, recorder.Code)

		var responseProduct models.Product
		err := json.Unmarshal(recorder.Body.Bytes(), &responseProduct)
		assert.NoError(t, err)
		assert.Greater(t, responseProduct.ID, uint(0))
		assert.Equal(t, "Laptop", responseProduct.Name)
		assert.Equal(t, 1200.00, responseProduct.Price)
		assert.Equal(t, category.ID, responseProduct.CategoryID)
		assert.NotNil(t, responseProduct.Category)
		assert.Equal(t, category.Name, responseProduct.Category.Name)

		// Verifying database state
		var storedProduct models.Product
		testDB.Preload("Category").First(&storedProduct, responseProduct.ID)
		assert.Equal(t, "Laptop", storedProduct.Name)
		assert.Equal(t, 1200.00, storedProduct.Price)
		assert.Equal(t, category.ID, storedProduct.CategoryID)
		assert.NotNil(t, storedProduct.Category)
		assert.Equal(t, category.Name, storedProduct.Category.Name)
	})

	t.Run("Returns 400 for invalid JSON request - missing name", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"price":       100.00,
			"category_id": category.ID,
		}
		recorder := httptest.NewRecorder()
		req := createProductRequest(http.MethodPost, "/api/products", reqBody)
		router.ServeHTTP(recorder, req)

		assert.Equal(t, http.StatusBadRequest, recorder.Code)
		var response map[string]string
		json.Unmarshal(recorder.Body.Bytes(), &response)
		assert.Contains(t, response["error"], "Key: 'CreateProductRequest.Name' Error:Field validation for 'Name' failed on the 'required' tag")
	})

	t.Run("Returns 400 for invalid JSON request - price less than or equal to 0", func(t *testing.T) {
		reqBody := handlers.CreateProductRequest{
			Name:       "Zero Price Item",
			Price:      0,
			CategoryID: category.ID,
		}
		recorder := httptest.NewRecorder()
		req := createProductRequest(http.MethodPost, "/api/products", reqBody)
		router.ServeHTTP(recorder, req)

		assert.Equal(t, http.StatusBadRequest, recorder.Code)
		var response map[string]string
		json.Unmarshal(recorder.Body.Bytes(), &response)
		assert.Contains(t, response["error"], "Key: 'CreateProductRequest.Price' Error:Field validation for 'Price' failed on the 'gt' tag")
	})

	t.Run("Returns 404 if category not found", func(t *testing.T) {
		nonExistentCategoryID := uint(999)
		reqBody := handlers.CreateProductRequest{
			Name:       "Product with Non-existent Category",
			Price:      50.00,
			CategoryID: nonExistentCategoryID,
		}
		recorder := httptest.NewRecorder()
		req := createProductRequest(http.MethodPost, "/api/products", reqBody)
		router.ServeHTTP(recorder, req)

		assert.Equal(t, http.StatusNotFound, recorder.Code)
		var response map[string]string
		json.Unmarshal(recorder.Body.Bytes(), &response)
		assert.Equal(t, fmt.Sprintf("Parent category not found with ID: %d", nonExistentCategoryID), response["error"])

		// Verify no product was created in DB
		var count int64
		testDB.Model(&models.Product{}).Where("name = ?", "Product with Non-existent Category").Count(&count)
		assert.Equal(t, int64(0), count)
	})

	t.Run("Returns 500 for database error during creation (simulated - harder in in-memory)", func(t *testing.T) {
		// This is generally hard to test directly in an in-memory SQLite with GORM
		// without mocking the GORM DB instance itself. For simpler handler tests,
		// we often skip direct DB error simulation and test it at the service layer
		// if a service layer exists.
		t.Skip("Skipping direct database error simulation for simplicity.")
	})
}

// TestGetAveragePriceHandler
func TestGetAveragePriceHandler(t *testing.T) {
	router, testDB := setupProductTestRouter(t)

	// Seed categories and products for testing GetAveragePrice
	cat1 := models.Category{Name: "Category 1"}
	cat2 := models.Category{Name: "Category 2"} // Child of Category 1 in mock
	cat3 := models.Category{Name: "Category 3"} // Child of Category 1 in mock
	cat4 := models.Category{Name: "Category 4"} // Independent category
	testDB.Create(&cat1)
	testDB.Create(&cat2)
	testDB.Create(&cat3)
	testDB.Create(&cat4)

	// Products for Category 1 (and its mocked children 2, 3)
	testDB.Create(&models.Product{Name: "P1.1", Price: 10.0, CategoryID: cat1.ID})
	testDB.Create(&models.Product{Name: "P1.2", Price: 20.0, CategoryID: cat1.ID})
	testDB.Create(&models.Product{Name: "P2.1", Price: 30.0, CategoryID: cat2.ID})
	testDB.Create(&models.Product{Name: "P3.1", Price: 40.0, CategoryID: cat3.ID})

	// Products for Category 4 (independent)
	testDB.Create(&models.Product{Name: "P4.1", Price: 50.0, CategoryID: cat4.ID})

	t.Run("Successfully gets average price for a category and its descendants", func(t *testing.T) {
		// With mock, category 1 should include products from 1, 2, and 3
		// (10+20+30+40)/4 = 100/4 = 25
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/products/average?category_id=%d", cat1.ID), nil)
		router.ServeHTTP(recorder, req)

		assert.Equal(t, http.StatusOK, recorder.Code)
		var response map[string]interface{}
		json.Unmarshal(recorder.Body.Bytes(), &response)
		assert.Equal(t, float64(cat1.ID), response["category_id"])
		assert.InDelta(t, 25.0, response["average_price"], 0.001) 
	})

	t.Run("Successfully gets average price for a category with no children (mocked)", func(t *testing.T) {
		// Category 4 only has P4.1 (50.0)
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/products/average_price?category_id=%d", cat4.ID), nil)
		router.ServeHTTP(recorder, req)

		assert.Equal(t, http.StatusOK, recorder.Code)
		var response map[string]interface{}
		json.Unmarshal(recorder.Body.Bytes(), &response)
		assert.Equal(t, float64(cat4.ID), response["category_id"])
		assert.InDelta(t, 50.0, response["average_price"], 0.001)
	})

	t.Run("Returns average price 0 for category with no products", func(t *testing.T) {
		// Create a new category with no products
		catNoProducts := models.Category{Name: "Category No Products"}
		testDB.Create(&catNoProducts)

		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/products/average_price?category_id=%d", catNoProducts.ID), nil)
		router.ServeHTTP(recorder, req)

		assert.Equal(t, http.StatusOK, recorder.Code)
		var response map[string]interface{}
		json.Unmarshal(recorder.Body.Bytes(), &response)
		assert.Equal(t, float64(catNoProducts.ID), response["category_id"])
		assert.InDelta(t, 0.0, response["average_price"], 0.001)
	})

	t.Run("Returns 400 if category_id is missing", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/products/average", nil) 
		router.ServeHTTP(recorder, req)

		assert.Equal(t, http.StatusBadRequest, recorder.Code)
		var response map[string]string
		json.Unmarshal(recorder.Body.Bytes(), &response)
		assert.Equal(t, "category_id is required", response["error"])
	})

	t.Run("Returns 400 for invalid category_id", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/products/average_price?category_id=abc", nil) // Non-numeric
		router.ServeHTTP(recorder, req)

		assert.Equal(t, http.StatusBadRequest, recorder.Code)
		var response map[string]string
		json.Unmarshal(recorder.Body.Bytes(), &response)
		assert.Equal(t, "Invalid category_id", response["error"])
	})

	t.Run("Returns 500 for database error during average calculation (simulated)", func(t *testing.T) {
		// This is also complex to simulate directly with in-memory GORM.
		// One way would be to close the database connection right before the query,
		// but that affects all subsequent tests. Better to mock the DB.
		t.Skip("Skipping direct database error simulation for average calculation for simplicity.")
	})
}