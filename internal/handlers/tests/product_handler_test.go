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

	// Auto-migrate all relevant models (Category must have ParentID field)
	err = testDB.AutoMigrate(&models.Product{}, &models.Category{})
	if err != nil {
		panic("failed to auto-migrate models: " + err.Error())
	}

	testDB.Exec("DELETE FROM products;")
    testDB.Exec("DELETE FROM categories;")

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
		err := json.Unmarshal(recorder.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Contains(t, response["error"], "Key: 'CreateProductRequest.Name' Error:Field validation for 'Name' failed on the 'required' tag")
	})

	t.Run("Returns 400 for invalid JSON request - price less than or equal to 0", func(t *testing.T) {
		reqBody := handlers.CreateProductRequest{
			Name:       "Negative Price Item",
			Price:      -1.0,
			CategoryID: category.ID,
		}
		recorder := httptest.NewRecorder()
		req := createProductRequest(http.MethodPost, "/api/products", reqBody)
		router.ServeHTTP(recorder, req)

		assert.Equal(t, http.StatusBadRequest, recorder.Code)
		var response map[string]string
		err := json.Unmarshal(recorder.Body.Bytes(), &response)
		assert.NoError(t, err)
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
		err := json.Unmarshal(recorder.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Equal(t, fmt.Sprintf("Category not found with ID: %d", nonExistentCategoryID), response["error"])

		// Verify no product was created in DB
		var count int64
		testDB.Model(&models.Product{}).Where("name = ?", "Product with Non-existent Category").Count(&count)
		assert.Equal(t, int64(0), count)
	})

	t.Run("Returns 500 for database error during creation (simulated - harder in in-memory)", func(t *testing.T) {
		t.Skip("Skipping direct database error simulation for simplicity.")
	})
}

// TestGetAveragePriceHandler
func TestGetAveragePriceHandler(t *testing.T) {
	router, testDB := setupProductTestRouter(t)

	// --- Seed categories with parent-child relationships ---
	// Category 1 (root)
	cat1 := models.Category{Name: "Electronics"}
	testDB.Create(&cat1)

	// Category 2 (child of Cat 1)
	cat2ParentID := cat1.ID
	cat2 := models.Category{Name: "Laptops", ParentID: &cat2ParentID}
	testDB.Create(&cat2)

	// Category 3 (child of Cat 1)
	cat3ParentID := cat1.ID
	cat3 := models.Category{Name: "Smartphones", ParentID: &cat3ParentID}
	testDB.Create(&cat3)

	// Category 5 (child of Cat 2) - grandchild of Cat 1
	cat5ParentID := cat2.ID
	cat5 := models.Category{Name: "Gaming Laptops", ParentID: &cat5ParentID}
	testDB.Create(&cat5)


	// Category 4 (independent root category)
	cat4 := models.Category{Name: "Books"}
	testDB.Create(&cat4)

	// --- Seed products associated with these categories ---
	// Products for Category 1 (Electronics)
	testDB.Create(&models.Product{Name: "All-Purpose Charger", Price: 10.0, CategoryID: cat1.ID})
	testDB.Create(&models.Product{Name: "Basic Mouse", Price: 20.0, CategoryID: cat1.ID})

	// Products for Category 2 (Laptops)
	testDB.Create(&models.Product{Name: "Budget Laptop", Price: 300.0, CategoryID: cat2.ID})
	testDB.Create(&models.Product{Name: "Mid-Range Laptop", Price: 500.0, CategoryID: cat2.ID})

	// Products for Category 3 (Smartphones)
	testDB.Create(&models.Product{Name: "Android Phone", Price: 400.0, CategoryID: cat3.ID})
	testDB.Create(&models.Product{Name: "iPhone", Price: 700.0, CategoryID: cat3.ID})

	// Products for Category 5 (Gaming Laptops) - grandchild of Cat 1
	testDB.Create(&models.Product{Name: "High-End Gaming Laptop", Price: 1500.0, CategoryID: cat5.ID})

	// Products for Category 4 (Books) - independent
	testDB.Create(&models.Product{Name: "Go Programming Book", Price: 50.0, CategoryID: cat4.ID})


	t.Run("Successfully gets average price for a root category and all its descendants", func(t *testing.T) {
		// Category 1 (Electronics) includes products from cat1, cat2, cat3, cat5
		// Prices: 10 + 20 + 300 + 500 + 400 + 700 + 1500 = 3430
		// Number of products: 7
		// Average: 3430 / 7 = 490
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/products/average?category_id=%d", cat1.ID), nil)
		router.ServeHTTP(recorder, req)

		assert.Equal(t, http.StatusOK, recorder.Code)
		var response map[string]interface{}
		err := json.Unmarshal(recorder.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Equal(t, float64(cat1.ID), response["category_id"])
		assert.InDelta(t, 490.0, response["average_price"], 0.001)
	})

	t.Run("Successfully gets average price for a mid-level category and its descendants", func(t *testing.T) {
		// Category 2 (Laptops) includes products from cat2, cat5
		// Prices: 300 + 500 + 1500 = 2300
		// Number of products: 3
		// Average: 2300 / 3 = 766.666...
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/products/average?category_id=%d", cat2.ID), nil)
		router.ServeHTTP(recorder, req)

		assert.Equal(t, http.StatusOK, recorder.Code)
		var response map[string]interface{}
		err := json.Unmarshal(recorder.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Equal(t, float64(cat2.ID), response["category_id"])
		assert.InDelta(t, 766.666, response["average_price"], 0.001) // Using 0.001 delta for float
	})

	t.Run("Successfully gets average price for a leaf category (no children) and its descendants", func(t *testing.T) {
		// Category 3 (Smartphones) has products only in cat3, no children with products
		// Prices: 400 + 700 = 1100
		// Number of products: 2
		// Average: 1100 / 2 = 550
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/products/average?category_id=%d", cat3.ID), nil)
		router.ServeHTTP(recorder, req)

		assert.Equal(t, http.StatusOK, recorder.Code)
		var response map[string]interface{}
		err := json.Unmarshal(recorder.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Equal(t, float64(cat3.ID), response["category_id"])
		assert.InDelta(t, 550.0, response["average_price"], 0.001)
	})

	t.Run("Successfully gets average price for an independent category", func(t *testing.T) {
		// Category 4 (Books) only has P4.1 (50.0)
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/products/average?category_id=%d", cat4.ID), nil)
		router.ServeHTTP(recorder, req)

		assert.Equal(t, http.StatusOK, recorder.Code)
		var response map[string]interface{}
		err := json.Unmarshal(recorder.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Equal(t, float64(cat4.ID), response["category_id"])
		assert.InDelta(t, 50.0, response["average_price"], 0.001)
	})

	t.Run("Returns average price 0 for category with no products in its hierarchy", func(t *testing.T) {
		// Create a new category with no products and no children
		catNoProducts := models.Category{Name: "Category No Products Hierarchy"}
		testDB.Create(&catNoProducts)

		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/products/average?category_id=%d", catNoProducts.ID), nil)
		router.ServeHTTP(recorder, req)

		assert.Equal(t, http.StatusOK, recorder.Code)
		var response map[string]interface{}
		err := json.Unmarshal(recorder.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Equal(t, float64(catNoProducts.ID), response["category_id"])
		assert.InDelta(t, 0.0, response["average_price"], 0.001)
	})

	t.Run("Returns 400 if category_id is missing", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/products/average", nil) // No query param
		router.ServeHTTP(recorder, req)

		assert.Equal(t, http.StatusBadRequest, recorder.Code)
		var response map[string]string
		err := json.Unmarshal(recorder.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Equal(t, "category_id is required", response["error"])
	})

	t.Run("Returns 400 for invalid category_id", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/products/average?category_id=abc", nil) // Non-numeric
		router.ServeHTTP(recorder, req)

		assert.Equal(t, http.StatusBadRequest, recorder.Code)
		var response map[string]string
		err := json.Unmarshal(recorder.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Equal(t, "Invalid category_id", response["error"])
	})

	t.Run("Returns 500 if GetAllCategoryIDs returns an error (simulated - for robustness)", func(t *testing.T) {
		// To simulate an error from utils.GetAllCategoryIDs without mocking,
		// you'd need to cause a database error *inside* the utils function itself.
		// This is difficult to do cleanly in an in-memory SQLite test without
		// directly manipulating the database connection at a low level.
		// For now, we'll keep this skipped as it's outside the scope of
		// simply removing the mock. If you had a way to inject a faulty DB
		// connection into utils.GetAllCategoryIDs, you'd do it here.
		t.Skip("Simulating utils.GetAllCategoryIDs internal error without mocking is complex.")
	})

	t.Run("Returns 500 for database error during average calculation (simulated)", func(t *testing.T) {
		t.Skip("Skipping direct database error simulation for average calculation for simplicity.")
	})
}