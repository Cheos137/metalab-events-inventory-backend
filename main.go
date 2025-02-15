package main

import (
	"fmt"
	"metalab/events-inventory-tracker/controllers"
	"metalab/events-inventory-tracker/models"
	"net/http"
	"os"
	"strings"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func main() {
	if os.Getenv("JWT_SECRET") == "" {
		panic("no JWT_SECRET specified in environment")
	}
	if os.Getenv("DB_USER") == "" || os.Getenv("DB_PASS") == "" {
		fmt.Println("no DB_USER or DB_PASS specified in environment, consider configuring DB_USER, DB_PASS, DB_NAME and DB_HOST")
	}
	router := gin.Default()
	
	cors_config := cors.DefaultConfig()
	cors_config.AllowAllOrigins = true
	cors_config.AddAllowHeaders("Authorization")
	router.Use(cors.New(cors_config))
	//router.Use(cors.Default()) // cors.Default() is equal to DefaultConfig with AllowAllOrigins set

	models.ConnectDatabase()

	router.POST("/api/items", validateSignedJwt("admin", "true"), controllers.CreateItem)
	router.GET("/api/items", controllers.FindItems)
	router.GET("/api/items/:id", controllers.FindItem)
	router.PATCH("/api/items/:id", validateSignedJwt("admin", "true"), controllers.UpdateItem)
	router.DELETE("/api/items/:id", validateSignedJwt("admin", "true"), controllers.DeleteItem)

	router.POST("/api/purchases", validateSignedJwt("iss", "metalab-events-backend"), controllers.CreatePurchase)
	router.GET("/api/purchases", controllers.FindPurchases)
	router.GET("/api/purchases/:id", controllers.FindPurchase)
	router.PATCH("/api/purchases/:id", validateSignedJwt("admin", "true"), controllers.UpdatePurchase)
	router.DELETE("/api/purchases/:id", validateSignedJwt("admin", "true"), controllers.DeletePurchase)

	router.POST("/api/users", validateSignedJwt("admin", "true"), controllers.CreateUser)
	router.GET("/api/users", validateSignedJwt("admin", "true"), controllers.FindUsers)
	router.GET("/api/users/:id", validateSignedJwt("admin", "true"), controllers.FindUser)
	router.PATCH("/api/users", validateSignedJwt("admin", "true"), controllers.UpdateUser)
	router.DELETE("/api/users", validateSignedJwt("admin", "true"), controllers.DeleteUser)

	router.GET("/api/token/validate", validateSignedJwt("iss", "metalab-events-backend"), controllers.ValidateToken)

	router.Run("0.0.0.0:8080")
}

func validateSignedJwt(claim string, value string) gin.HandlerFunc {
	return func(c *gin.Context) {
		auth_header := c.GetHeader("Authorization")
		if auth_header == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "Missing Authorization header"})
			return
		}
		if !strings.Contains(auth_header, " ") {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"message": "Malformed Authorization header"})
			return
		}
		// split header so jwt can be taken out
		parsed_jwt := strings.Split(auth_header, " ")
		// check if jwt in header is valid
		token, err := jwt.Parse(parsed_jwt[1], func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"message": "Wrong Bearer algorithm"})
				return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
			}
			return []byte(os.Getenv("JWT_SECRET")), nil
		})
		if err != nil {
			// if jwt parser returns an error
			fmt.Println(err)
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// check if token is in db
		var user models.User
		if err := models.DB.Where("token = ?", parsed_jwt[1]).First(&user).Error; err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}

		// check if claims are ok
		if claims, ok := token.Claims.(jwt.MapClaims); ok {
			// check if queried claims are in jwt claims
			if claims[claim] == value {
				c.Next()
				return
			} else {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Forbidden"})
				return
			}
		} else {
			// if mapping claims returns an error
			fmt.Println(err)
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Error while parsing claims, JWT content may be malformed"})
			return
		}
	}
}
