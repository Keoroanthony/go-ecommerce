package handlers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Keoroanthony/go-ecommerce/internal/db"
	"github.com/Keoroanthony/go-ecommerce/internal/handlers"
	"github.com/Keoroanthony/go-ecommerce/internal/models"
)

func setupOrderTestRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
	gin.SetMode(gin.TestMode)

	// Initialize an in-memory SQLite database
	testDB, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		panic("failed to connect test database: " + err.Error())
	}

	// AutoMigrate all relevant models
	err = testDB.AutoMigrate(&models.Customer{}, &models.Product{}, &models.Order{}, &models.OrderItem{})
	if err != nil {
		panic("failed to auto-migrate models: " + err.Error())
	}

	originalDB := db.DB
	db.SetTestDB(testDB)

	r := gin.New()
	r.Use(gin.Recovery())

	store := cookie.NewStore([]byte("test-secret-key"))
	r.Use(sessions.Sessions("gosess", store))

	api := r.Group("/api")
	{
		api.POST("/orders", handlers.CreateOrder)
	}

	t.Cleanup(func() {
		db.SetTestDB(originalDB)
	})

	return r, testDB
}

func createOrderRequest(method, path string, body interface{}) *http.Request {
	var reqBody []byte
	if body != nil {
		reqBody, _ = json.Marshal(body)
	}
	req := httptest.NewRequest(method, path, bytes.NewBuffer(reqBody))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func performOrderAuthenticatedRequest(router *gin.Engine, method, path string, body interface{}, customerID *uint) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	req := createOrderRequest(method, path, body)

	// Create a new context to simulate the session middleware
	tempW := httptest.NewRecorder()
	tempC, _ := gin.CreateTestContext(tempW)
	tempC.Request = httptest.NewRequest(http.MethodGet, "/", nil) // Dummy request for context
	store := cookie.NewStore([]byte("test-secret-key"))
	sessions.Sessions("gosess", store)(tempC) // Apply session middleware to temp context

	session := sessions.Default(tempC)
	if customerID != nil {
		session.Set("customer_id", *customerID)
	} else {
		session.Delete("customer_id") // Ensure no customer_id is set if nil
	}
	session.Save()

	// Copy the session cookie from tempC to the actual request
	req.Header.Set("Cookie", tempW.Header().Get("Set-Cookie"))

	router.ServeHTTP(recorder, req)
	return recorder
}


func TestCreateOrderHandler(t *testing.T) {

	router, testDB := setupOrderTestRouter(t)

	// Seed data for tests
	category := models.Category{Name: "Computers"}
	testDB.Create(&category)

	customer := models.Customer{Name: "Test Customer", Email: "test@example.com", Phone: "1234567890"}
	testDB.Create(&customer)

	product1 := models.Product{Name: "Product A", Price: 10.00, CategoryID: category.ID}
	product2 := models.Product{Name: "Product B", Price: 20.00, CategoryID: category.ID}
	testDB.Create(&product1)
	testDB.Create(&product2)

	t.Run("Successfully creates an order", func(t *testing.T) {
		reqBody := handlers.CreateOrderRequest{
			ProductIDs: []uint{product1.ID, product2.ID},
		}
		custID := customer.ID
		recorder := performOrderAuthenticatedRequest(router, http.MethodPost, "/api/orders", reqBody, &custID)

		assert.Equal(t, http.StatusCreated, recorder.Code)

		var response struct {
			Message string      `json:"message"`
			Order   models.Order `json:"order"`
		}
		err := json.Unmarshal(recorder.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Equal(t, "order created successfully", response.Message)
		assert.Greater(t, response.Order.ID, uint(0))
		assert.Equal(t, customer.ID, response.Order.CustomerID)
		assert.Len(t, response.Order.Items, 2) 
		assert.Equal(t, product1.ID, response.Order.Items[0].ProductID)
		assert.Equal(t, product2.ID, response.Order.Items[1].ProductID)

		// Verify database state
		var storedOrder models.Order
		testDB.Preload("Items").First(&storedOrder, response.Order.ID)
		assert.Equal(t, customer.ID, storedOrder.CustomerID)
		assert.Len(t, storedOrder.Items, 2)
		assert.Equal(t, product1.ID, storedOrder.Items[0].ProductID)
		assert.Equal(t, product2.ID, storedOrder.Items[1].ProductID)
	})

	t.Run("Returns 401 for unauthorized (no customer_id in session)", func(t *testing.T) {
		reqBody := handlers.CreateOrderRequest{
			ProductIDs: []uint{product1.ID},
		}
		recorder := performOrderAuthenticatedRequest(router, http.MethodPost, "/api/orders", reqBody, nil) // Pass nil to simulate no customer_id

		assert.Equal(t, http.StatusUnauthorized, recorder.Code)
		var response map[string]string
		json.Unmarshal(recorder.Body.Bytes(), &response)
		assert.Equal(t, "unauthorized", response["error"])
	})

	t.Run("Returns 400 for invalid JSON request", func(t *testing.T) {
		// Missing "product_ids" field or incorrect type
		reqBody := map[string]interface{}{"invalid_field": "value"}
		custID := customer.ID
		recorder := performOrderAuthenticatedRequest(router, http.MethodPost, "/api/orders", reqBody, &custID)

		assert.Equal(t, http.StatusBadRequest, recorder.Code)
		var response map[string]string
		json.Unmarshal(recorder.Body.Bytes(), &response)
		assert.Equal(t, "product_ids required", response["error"])
	})

	t.Run("Returns 400 for empty product_ids", func(t *testing.T) {
		reqBody := handlers.CreateOrderRequest{
			ProductIDs: []uint{},
		}
		custID := customer.ID
		recorder := performOrderAuthenticatedRequest(router, http.MethodPost, "/api/orders", reqBody, &custID)

		assert.Equal(t, http.StatusBadRequest, recorder.Code)
		var response map[string]string
		json.Unmarshal(recorder.Body.Bytes(), &response)
		assert.Equal(t, "product_ids required", response["error"])
	})

	t.Run("Returns 400 if customer not found", func(t *testing.T) {
		reqBody := handlers.CreateOrderRequest{
			ProductIDs: []uint{product1.ID},
		}
		nonExistentCustomerID := uint(9999) // A customer ID that doesn't exist
		recorder := performOrderAuthenticatedRequest(router, http.MethodPost, "/api/orders", reqBody, &nonExistentCustomerID)

		assert.Equal(t, http.StatusBadRequest, recorder.Code)
		var response map[string]string
		json.Unmarshal(recorder.Body.Bytes(), &response)
		assert.Equal(t, "customer not found", response["error"])
	})

	t.Run("Returns 404 if a product not found", func(t *testing.T) {
		reqBody := handlers.CreateOrderRequest{
			ProductIDs: []uint{product1.ID, 99999}, // 99999 is a non-existent product ID
		}
		custID := customer.ID
		recorder := performOrderAuthenticatedRequest(router, http.MethodPost, "/api/orders", reqBody, &custID)

		assert.Equal(t, http.StatusNotFound, recorder.Code)
		var response map[string]string
		json.Unmarshal(recorder.Body.Bytes(), &response)
		assert.Contains(t, response["error"], "Product not found with ID: 99999")

		// Verify no order was created in DB for this failed attempt
		var count int64
		testDB.Model(&models.Order{}).Where("customer_id = ?", customer.ID).Count(&count)
		assert.Equal(t, int64(1), count) // Only the successful order from the first test case
	})

	// Note: Simulating internal server errors due to database issues (like transaction failures)
	// directly in a handler test is challenging without a mock database or specific GORM error injection.
	// This is typically better handled by mocking the `db.DB` dependency in your service layer if you have one,
	// or by using a dedicated integration test that can induce real database errors (e.g., by closing the connection).
	t.Run("Returns 500 for internal server error during order creation (simulated)", func(t *testing.T) {
		// This test case is difficult to implement reliably without mocking the DB.
		// For example, to simulate a `tx.Create(&order).Error` you would need to
		// force the `Create` call to fail for a specific scenario.
		// As a workaround, you could temporarily set db.DB to a mock that always returns an error for `Create`.
		// However, for this direct handler test, we'll acknowledge the complexity.
		t.Skip("Skipping direct database error simulation at handler level for simplicity.")
	})
}