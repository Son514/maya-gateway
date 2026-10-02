package main

// Epic 2: Webhooks
// TODO: US-201: Expose your local server with ngrok
// TODO: US-202: Register a webhook with Maya
// TODO: US-203: Receive webhook callbacks
// TODO: US-204: Update database from webhook

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	"son514/maya-gateway/maya-gateway/db"
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

// buyerName joins the buyer name parts Maya accepts, skipping absent ones.
func buyerName(b *models.Buyer) string {
	if b == nil {
		return ""
	}
	parts := make([]string, 0, 3)
	for _, p := range []string{b.FirstName, b.MiddleName, b.LastName} {
		if p = strings.TrimSpace(p); p != "" {
			parts = append(parts, p)
		}
	}
	return strings.Join(parts, " ")
}

/* --- Perform checkout via maya --- */
func checkoutMaya(maya *models.Maya, pool *sql.DB) gin.HandlerFunc {
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

		// The checkout exists at Maya whether or not we record it, so a database
		// failure must not fail the request.
		amount, currency := 0.0, ""
		if req.TotalAmount != nil {
			amount, currency = req.TotalAmount.Value, req.TotalAmount.Currency
		}
		if err := db.InsertCheckout(c.Request.Context(), pool, db.Transaction{
			CheckoutID:  checkout.CheckoutID,
			Amount:      amount,
			Currency:    currency,
			BuyerName:   buyerName(req.Buyer),
			RedirectURL: checkout.RedirectURL,
		}); err != nil {
			log.Printf("record checkout %s: %v", checkout.CheckoutID, err)
		}

		c.JSON(http.StatusOK, checkout)
	}
}

/* --- Verify the maya checkout status --- */
func checkoutStatusMaya(maya *models.Maya, pool *sql.DB) gin.HandlerFunc {
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

		// Maya's answer is current whether or not we record it, so a database
		// failure must not fail the read. Keyed on the ID Maya echoed rather than
		// the URL parameter, so the update cannot land on a different row.
		if ok, err := db.UpdateStatus(c.Request.Context(), pool, checkout.ID, checkout.PaymentStatus); err != nil {
			log.Printf("update status %s: %v", checkout.ID, err)
		} else if !ok {
			log.Printf("update status %s: no local row, skipped", checkout.ID)
		}

		c.JSON(http.StatusOK, checkout)
	}
}

func main() {
	if err := godotenv.Load(); err != nil {
		log.Fatalf("load .env: %v", err)
	}

	/* --- Initialize Maya --- */
	maya := models.NewMaya(
		os.Getenv("MAYA_BASE_URL"),
		os.Getenv("MAYA_PUBLIC_KEY"),
		os.Getenv("MAYA_SECRET_KEY"),
	)

	/* --- Initialize Database --- */
	cfg := db.ConfigFromEnv()

	pool, err := db.Connect(context.Background(), cfg)
	if err != nil {
		log.Fatalf("connect database: %v", err)
	}
	defer pool.Close()

	if err := db.MigrateUp(cfg, "db/migrations"); err != nil {
		log.Fatalf("migrate database: %v", err)
	}

	if err := db.RequireTable(context.Background(), pool, "transactions"); err != nil {
		log.Fatalf("schema: %v", err)
	}

	router := gin.Default()

	checkout := router.Group("/checkout")
	checkout.POST("/v1/checkouts", checkoutMaya(maya, pool))
	checkout.GET("/v1/checkouts/:id", checkoutStatusMaya(maya, pool))

	result := servePage("result.html")
	router.GET("/", servePage("index.html"))
	router.GET("/success", result)
	router.GET("/failure", result)
	router.GET("/cancel", result)

	router.Run()
}
