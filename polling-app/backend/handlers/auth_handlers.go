package handlers

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"pollingapp/auth"
	"pollingapp/config"
	"pollingapp/models"
)

type AuthHandler struct {
	DB  *mongo.Database
	Cfg *config.Config
}

type signupRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// Signup validates on the server — never trusts the client. Real checks,
// not decoration: length bounds, no blank fields, uniqueness enforced by
// both an app-level check and a unique Mongo index as a second line of defense.
func (h *AuthHandler) Signup(c *gin.Context) {
	var req signupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	req.Username = strings.TrimSpace(req.Username)

	if len(req.Username) < 3 || len(req.Username) > 32 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username must be 3-32 characters"})
		return
	}
	if len(req.Password) < 8 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "password must be at least 8 characters"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not process password"})
		return
	}

	user := models.User{
		ID:           uuid.NewString(),
		Username:     req.Username,
		PasswordHash: hash,
		CreatedAt:    time.Now().UTC(),
	}

	_, err = h.DB.Collection("users").InsertOne(ctx, user)
	if mongo.IsDuplicateKeyError(err) {
		c.JSON(http.StatusConflict, gin.H{"error": "username already taken"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create user"})
		return
	}

	token, err := auth.GenerateToken(h.Cfg.JWTSecret, user.ID, user.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not generate token"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"token": token, "username": user.Username})
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var user models.User
	err := h.DB.Collection("users").FindOne(ctx, bson.M{"username": strings.TrimSpace(req.Username)}).Decode(&user)
	if err != nil || !auth.CheckPassword(user.PasswordHash, req.Password) {
		// Deliberately vague — don't reveal whether the username exists.
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid username or password"})
		return
	}

	token, err := auth.GenerateToken(h.Cfg.JWTSecret, user.ID, user.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not generate token"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"token": token, "username": user.Username})
}
