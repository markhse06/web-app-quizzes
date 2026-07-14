package app

import (
	"Backend/internal/config"
	"Backend/internal/domain"
	"Backend/internal/domain/backend_entities"
	"Backend/internal/dto"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var cfg = config.LoadJWT()
var jwtSecret = []byte(cfg.Secret)

func (a *App) handleUserRegistration(c *gin.Context) {
	var req dto.UserRegistrationRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		a.logger.Error("failed to parse registration request", "error", err)
		return
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		a.logger.Error("failed to generate password", "error", err)
		return
	}

	var user = domain.User{
		Email:        req.Email,
		PasswordHash: string(passwordHash),
	}

	if err := a.db.Create(&user).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "failed to create user", "details": err.Error()})
		a.logger.Error("failed to create user", "error", err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{"id": user.ID, "email": user.Email})
}

func (a *App) handleUserLogin(c *gin.Context) {
	var req dto.UserLoginRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		a.logger.Error("failed to parse login request", "error", err)
		return
	}

	u := domain.User{}
	if err := a.db.Where("email = ?", req.Email).First(&u).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "email not found"})
		a.logger.Error("failed to find user", "error", err)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "password incorrect"})
		a.logger.Error("password incorrect", "error", err)
		return
	}

	accessToken, refreshToken, err := generateJWT(u.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate access token", "details": err.Error()})
		a.logger.Error("failed to generate access token", "error", err)
		return
	}

	refresh := backend_entities.Token{
		UserID:    u.ID,
		Secret:    refreshToken,
		ExpiresAt: time.Now().Add(time.Hour * 6),
	}

	if err := a.db.Create(&refresh).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create refresh token", "details": err.Error()})
		a.logger.Error("failed to create refresh token", "error", err)
		return
	}

	res := dto.UserLoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User: dto.User{
			ID:    u.ID.String(),
			Email: u.Email,
		},
	}

	c.JSON(http.StatusOK, res)
}

func (a *App) handleRefreshToken(c *gin.Context) {
	var req dto.UserRefreshTokenRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		a.logger.Error("failed to parse refresh token request", "error", err)
		return
	}

	var storedToken backend_entities.Token
	if err := a.db.Where("secret = ?", req.RefreshToken).First(&storedToken).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "refresh token not found"})
		a.logger.Error("failed to find token", "error", err)
		return
	}

	if storedToken.ExpiresAt.Before(time.Now()) {
		a.db.Delete(&storedToken)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "refresh token is expired"})
		a.logger.Error("refresh token is expired")
		return
	}

	user := domain.User{}
	if err := a.db.First(&user, "id = ?", storedToken.UserID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		a.logger.Error("failed to find user", "error", err)
		return
	}

	accessToken, refreshToken, err := generateJWT(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate access token", "details": err.Error()})
		a.logger.Error("failed to generate access token", "error", err)
		return
	}

	err = a.db.Transaction(func(tx *gorm.DB) error {
		tx.Delete(&storedToken)

		token := backend_entities.Token{
			UserID:    user.ID,
			Secret:    refreshToken,
			ExpiresAt: time.Now().Add(time.Hour * 6),
		}

		return a.db.Create(&token).Error
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create refresh token", "details": err.Error()})
		a.logger.Error("failed to create refresh token", "error", err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
	})
}
