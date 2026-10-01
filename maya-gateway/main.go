package main

// TODO: US-203: Call Maya's "Get Checkout Status" API
import (
	"embed"
	"errors"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	"son514/maya-gateway/maya-gateway/models"
)

//go:embed web
var webFS embed.FS

func servePage(name string) gin.HandlerFunc {
	return func(c *gin.Context) {
		page, err := webFS.ReadFile("web/" + name)
		if err != nil {
			c.String(http.StatusInternalServerError, "read %s: %v", name, err)
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", page)
	}
}

func checkoutMaya(maya *models.Maya) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req models.CreateCheckoutRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		checkout, err := maya.CreateCheckout(c.Request.Context(), req)
		if err != nil {
			var apiErr *models.APIError
			if errors.As(err, &apiErr) {
				c.JSON(apiErr.StatusCode, gin.H{"error": apiErr.Body})
				return
			}
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, checkout)
	}
}

func checkoutStatusMaya(maya *models.Maya) gin.HandlerFunc {
	return func(c *gin.Context) {
		checkout, err := maya.GetCheckout(c.Request.Context(), c.Param("id"))
		if err != nil {
			var apiErr *models.APIError
			if errors.As(err, &apiErr) {
				c.JSON(apiErr.StatusCode, gin.H{"error": apiErr.Body})
				return
			}
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, checkout)
	}
}

func main() {
	if err := godotenv.Load(); err != nil {
		log.Fatalf("load .env: %v", err)
	}

	maya := models.NewMaya(
		os.Getenv("MAYA_BASE_URL"),
		os.Getenv("MAYA_PUBLIC_KEY"),
		os.Getenv("MAYA_SECRET_KEY"),
	)

	router := gin.Default()

	checkout := router.Group("/checkout")
	checkout.POST("/v1/checkouts", checkoutMaya(maya))
	checkout.GET("/v1/checkouts/:id", checkoutStatusMaya(maya))

	result := servePage("result.html")
	router.GET("/", servePage("index.html"))
	router.GET("/success", result)
	router.GET("/failure", result)
	router.GET("/cancel", result)

	router.Run()
}
