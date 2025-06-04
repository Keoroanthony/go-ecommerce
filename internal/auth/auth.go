package auth

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"

	"github.com/Keoroanthony/go-ecommerce/internal/db"
	"github.com/Keoroanthony/go-ecommerce/internal/models"
)

var (
	provider     *oidc.Provider
	verifier     *oidc.IDTokenVerifier
	oauth2Config *oauth2.Config
)

const sessionName = "gosess"

func Init() {
	ctx := context.Background()

	var err error
	provider, err = oidc.NewProvider(ctx, os.Getenv("OIDC_ISSUER"))
	if err != nil {
		log.Fatalf("OIDC provider init error: %v", err)
	}

	verifier = provider.Verifier(&oidc.Config{ClientID: os.Getenv("OIDC_CLIENT_ID")})

	oauth2Config = &oauth2.Config{
		ClientID:     os.Getenv("OIDC_CLIENT_ID"),
		ClientSecret: os.Getenv("OIDC_CLIENT_SECRET"),
		RedirectURL:  os.Getenv("OIDC_REDIRECT_URL"),
		Endpoint:     provider.Endpoint(),
		Scopes:       []string{oidc.ScopeOpenID, "profile", "email", "phone"},
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Handlers
// ─────────────────────────────────────────────────────────────────────────────

// GET /auth/login
func Login(c *gin.Context) {
	state := "rand" // TODO: generate & store real CSRF-safe state if needed
	url := oauth2Config.AuthCodeURL(state)
	c.Redirect(http.StatusFound, url)
}

// GET /auth/callback
func Callback(c *gin.Context) {
	code := c.Query("code")
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "code missing"})
		return
	}

	ctx := c.Request.Context()
	oauth2Token, err := oauth2Config.Exchange(ctx, code)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "token exchange failed"})
		return
	}

	rawIDToken, ok := oauth2Token.Extra("id_token").(string)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no id_token in token response"})
		return
	}

	idToken, err := verifier.Verify(ctx, rawIDToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "token verification failed"})
		return
	}

	// Extract claims
	var claims struct {
		Sub   string `json:"sub"`
		Name  string `json:"name"`
		Email string `json:"email"`
		Phone string `json:"phone_number"`
	}
	if err := idToken.Claims(&claims); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "claims parse error"})
		return
	}

	// Upsert customer
	var cust models.Customer
	if err := db.DB.Where("o_id_c_id = ?", claims.Sub).First(&cust).Error; err != nil {
		cust = models.Customer{
			OIDCID: claims.Sub,
			Name:   claims.Name,
			Email:  claims.Email,
			Phone:  claims.Phone,
		}
		db.DB.Create(&cust)
	}

	// Store customer-ID in session
	sess := sessions.Default(c)
	sess.Set("customer_id", cust.ID)
	_ = sess.Save()

	c.JSON(http.StatusOK, gin.H{"message": "logged in", "customer": cust})
}

// Middleware: ensures user is logged in and injects *models.Customer into context.
func RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		sess := sessions.Default(c)
		custID, ok := sess.Get("customer_id").(uint)
		if !ok || custID == 0 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		var cust models.Customer
		if err := db.DB.First(&cust, custID).Error; err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
			return
		}
		// put on context for handlers
		c.Set("customer", &cust)
		c.Next()
	}
}
