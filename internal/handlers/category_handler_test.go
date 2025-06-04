package handlers_test

import (
	"bytes"
	"encoding/json"
	"fmt"
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

func setupCategoryTestRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
	gin.SetMode(gin.TestMode)

	// Initialize an in-memory SQLite database
	testDB, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		panic("failed to connect test database: " + err.Error())
	}

	err = testDB.AutoMigrate(&models.Category{})
	if err != nil {
		panic("failed to auto-migrate models: " + err.Error())
	}

	originalDB := db.DB // Store the original DB instance before setting test DB
	db.SetTestDB(testDB)

	r := gin.New()
	r.Use(gin.Recovery())

	store := cookie.NewStore([]byte("test-secret-key"))
	r.Use(sessions.Sessions("gosess", store)) // The session middleware is applied here

	api := r.Group("/api")
	{
		api.POST("/categories", handlers.CreateCategory)
	}

	// This t.Cleanup now correctly refers to the t passed into the function
	t.Cleanup(func() {
		db.SetTestDB(originalDB) // Restore the original DB after the test or subtest finishes
	})

	return r, testDB
}

func createCategoryRequest(method, path string, body interface{}) *http.Request {
	var reqBody []byte
	if body != nil {
		reqBody, _ = json.Marshal(body)
	}
	req := httptest.NewRequest(method, path, bytes.NewBuffer(reqBody))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func performCategoryAuthenticatedRequest(router *gin.Engine, method, path string, body interface{}, customerID uint) *httptest.ResponseRecorder {
    recorder := httptest.NewRecorder()
    req := createCategoryRequest(method, path, body)

    tempW := httptest.NewRecorder()
    tempC, _ := gin.CreateTestContext(tempW)
    tempC.Request = httptest.NewRequest(http.MethodGet, "/", nil)
    store := cookie.NewStore([]byte("test-secret-key"))
    sessions.Sessions("gosess", store)(tempC)
    session := sessions.Default(tempC)
    session.Set("customer_id", customerID)
    session.Save()
    req.Header.Set("Cookie", tempW.Header().Get("Set-Cookie")) 

    router.ServeHTTP(recorder, req)
    return recorder
}


func TestCreateCategoryHandler(t *testing.T) {
	// Pass t to setupTestRouter
	router, testDB := setupCategoryTestRouter(t)

	t.Run("Successfully creates a top-level category", func(t *testing.T) {
		reqBody := handlers.CreateCategoryRequest{Name: "Electronics"}
		recorder := performCategoryAuthenticatedRequest(router, http.MethodPost, "/api/categories", reqBody, 1)

		assert.Equal(t, http.StatusCreated, recorder.Code)

		var responseCategory models.Category
		err := json.Unmarshal(recorder.Body.Bytes(), &responseCategory)
		assert.NoError(t, err)
		assert.Greater(t, responseCategory.ID, uint(0))
		assert.Equal(t, "Electronics", responseCategory.Name)
		assert.Nil(t, responseCategory.ParentID)
		assert.Nil(t, responseCategory.Parent) // Parent should be nil for top-level

		// Verify database state
		var storedCategory models.Category
		testDB.First(&storedCategory, responseCategory.ID)
		assert.Equal(t, "Electronics", storedCategory.Name)
		assert.Nil(t, storedCategory.ParentID)
	})

	t.Run("Successfully creates a sub-category with a valid parent", func(t *testing.T) {
		// First, create a parent category directly in DB for testing
		parentCategory := models.Category{Name: "Computers"}
		testDB.Create(&parentCategory)

		reqBody := handlers.CreateCategoryRequest{Name: "Laptops", ParentID: &parentCategory.ID}
		recorder := performCategoryAuthenticatedRequest(router, http.MethodPost, "/api/categories", reqBody, 1)

		assert.Equal(t, http.StatusCreated, recorder.Code)

		var responseCategory models.Category
		err := json.Unmarshal(recorder.Body.Bytes(), &responseCategory)
		assert.NoError(t, err)
		assert.Greater(t, responseCategory.ID, uint(0))
		assert.Equal(t, "Laptops", responseCategory.Name)
		assert.NotNil(t, responseCategory.ParentID)
		assert.Equal(t, parentCategory.ID, *responseCategory.ParentID)
		assert.NotNil(t, responseCategory.Parent) // Parent should be preloaded
		assert.Equal(t, parentCategory.Name, responseCategory.Parent.Name)

		// Verify database state
		var storedCategory models.Category
		testDB.Preload("Parent").First(&storedCategory, responseCategory.ID)
		assert.Equal(t, "Laptops", storedCategory.Name)
		assert.NotNil(t, storedCategory.ParentID)
		assert.Equal(t, parentCategory.ID, *storedCategory.ParentID)
		assert.NotNil(t, storedCategory.Parent)
		assert.Equal(t, parentCategory.Name, storedCategory.Parent.Name)
	})

	t.Run("Returns 400 for invalid JSON request", func(t *testing.T) {
		// Missing "name" field which is "required"
		reqBody := map[string]interface{}{"parent_id": 1}
		recorder := performCategoryAuthenticatedRequest(router, http.MethodPost, "/api/categories", reqBody, 1)

		assert.Equal(t, http.StatusBadRequest, recorder.Code)
		var response map[string]string
		json.Unmarshal(recorder.Body.Bytes(), &response)
		assert.Contains(t, response["error"], "Key: 'CreateCategoryRequest.Name' Error:Field validation for 'Name' failed on the 'required' tag")
	})

	t.Run("Returns 404 if parent category not found", func(t *testing.T) {
		nonExistentParentID := uint(999)
		reqBody := handlers.CreateCategoryRequest{Name: "NonExistentSubCategory", ParentID: &nonExistentParentID}
		recorder := performCategoryAuthenticatedRequest(router, http.MethodPost, "/api/categories", reqBody, 1)

		assert.Equal(t, http.StatusNotFound, recorder.Code)
		var response map[string]string
		json.Unmarshal(recorder.Body.Bytes(), &response)
		assert.Equal(t, fmt.Sprintf("Parent category not found with ID: %d", nonExistentParentID), response["error"])

		// Verify no category was created in DB
		var count int64
		testDB.Model(&models.Category{}).Where("name = ?", "NonExistentSubCategory").Count(&count)
		assert.Equal(t, int64(0), count)
	})

	t.Run("Returns 500 for database error during creation (simulated)", func(t *testing.T) {
		// This is harder to simulate directly with an in-memory DB without
		// injecting a mock GORM DB that can be configured to return errors.
		// For simplicity in this direct handler test, we'll skip direct DB error simulation.
		// In a service layer test, you would mock the service's DB dependency.
		t.Skip("Skipping direct database error simulation in handler test for simplicity.")
	})
}