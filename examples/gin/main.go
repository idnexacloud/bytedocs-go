package main

import (
	"log"
	"strings"

	"github.com/aibnuhibban/bytedocs/pkg/core"
	_ "github.com/aibnuhibban/bytedocs/pkg/llm"
	"github.com/aibnuhibban/bytedocs/pkg/parser"
	"github.com/gin-gonic/gin"
)

type User struct {
	ID    int    `json:"id" example:"123"`
	Name  string `json:"name" example:"John Doe"`
	Email string `json:"email" example:"john@example.com"`
}

type Product struct {
	ID          int     `json:"id" example:"1"`
	Name        string  `json:"name" example:"iPhone 14"`
	Price       float64 `json:"price" example:"999.99"`
	Description string  `json:"description" example:"Latest iPhone model"`
}

func main() {
	r := gin.Default()

	config, err := core.LoadConfigFromEnv()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	if err := core.ValidateConfig(config); err != nil {
		log.Fatalf("Invalid config: %v", err)
	}

	if config.Title == "API Documentation" {
		config.Title = "Gin API Documentation"
	}
	if config.DocsPath == "" {
		config.DocsPath = "/docs"
	}
	if !config.AutoDetect {
		config.AutoDetect = true
	}


	parser.SetupGinDocs(r, config)

	api := r.Group("/api/v1")
	{
		users := api.Group("/users")
		{
			users.GET("", GetUsers)
			users.POST("", CreateUser)
			users.GET("/:id", GetUser)
			users.PUT("/:id", UpdateUser)
		
			users.PATCH(":id", PatchUser)
			users.DELETE("/:id", DeleteUser)
		}

		products := api.Group("/products")
		{
			products.GET("", GetProducts)
			products.POST("", CreateProduct)
			products.GET("/:id", GetProduct)

			products.PATCH(":id", PatchProduct)
		}
	}

	port := ":8083"
	log.Printf("🚀 %s starting on %s", config.Title, port)
	log.Printf("📚 API Documentation: http://localhost%s%s", port, config.DocsPath)

	if config.AuthConfig != nil && config.AuthConfig.Enabled {
		switch config.AuthConfig.Type {
		case "session":
			log.Println("🔐 Laravel-style Session Authentication enabled")
			log.Printf("   📝 Password: %s", config.AuthConfig.Password)
			log.Printf("   ⏰ Session expires: %d minutes", config.AuthConfig.SessionExpire)
			if config.AuthConfig.IPBanEnabled {
				log.Printf("   🛡️  IP banning: Max %d attempts, %d min ban", config.AuthConfig.IPBanMaxAttempts, config.AuthConfig.IPBanDuration)
			}
		case "basic":
			log.Printf("🔐 Basic Authentication enabled (%s:%s)", config.AuthConfig.Username, config.AuthConfig.Password)
		case "api_key", "bearer":
			log.Printf("🔐 %s Authentication enabled", strings.ToUpper(config.AuthConfig.Type))
		}
	} else {
		log.Println("🌐 No authentication required")
	}

	if config.AIConfig != nil && config.AIConfig.Enabled {
		log.Printf("💬 AI Chat enabled (%s - %s)", config.AIConfig.Provider, config.AIConfig.Features.Model)
	}

	log.Printf("📋 OpenAPI spec: http://localhost%s%s/openapi.json", port, config.DocsPath)
	log.Println("")

	r.Run(port)
}

// GetUsers retrieves a list of all users with pagination support
// @Param page query int false "Page number for pagination (default: 1)"
// @Param limit query int false "Number of users per page (default: 10, max: 100)"
// @Param search query string false "Search term to filter users by name or email"
func GetUsers(c *gin.Context) {
	users := []User{
		{ID: 1, Name: "John Doe", Email: "john@example.com"},
		{ID: 2, Name: "Jane Smith", Email: "jane@example.com"},
	}
	c.JSON(200, gin.H{"users": users})
}

func CreateUser(c *gin.Context) {
	var user User
	c.ShouldBindJSON(&user)
	user.ID = 123
	c.JSON(201, user)
}

// GetUser retrieves detailed information about a specific user
// @Param id path int true "User ID to retrieve"
func GetUser(c *gin.Context) {
	_ = c.Param("id")
	user := User{
		ID:    123,
		Name:  "John Doe",
		Email: "john@example.com",
	}
	c.JSON(200, user)
}

// UpdateUser updates user information (requires authentication)
// @Param id path int true "User ID to update"
func UpdateUser(c *gin.Context) {
	var user User
	c.ShouldBindJSON(&user)
	c.JSON(200, user)
}

// PatchUser partially updates a user. Accepts a JSON object with any subset of fields.
func PatchUser(c *gin.Context) {
	id := c.Param("id")
	var updates map[string]interface{}
	if err := c.BindJSON(&updates); err != nil {
		c.JSON(400, gin.H{"error": "invalid request body"})
		return
	}

	user := User{
		ID:    123,
		Name:  "John Doe",
		Email: "john@example.com",
	}

	if v, ok := updates["name"].(string); ok {
		user.Name = v
	}
	if v, ok := updates["email"].(string); ok {
		user.Email = v
	}

	c.JSON(200, gin.H{"id": id, "user": user})
}

// DeleteUser permanently deletes a user account
// @Param id path int true "User ID to delete"
func DeleteUser(c *gin.Context) {
	c.JSON(204, nil)
}

// GetProducts retrieves all products with filtering options
// @Param category query string false "Filter products by category (e.g., Electronics, Clothing)"
// @Param min_price query number false "Minimum price filter"
// @Param max_price query number false "Maximum price filter"
// @Param sort query string false "Sort by: name, price, created_at (default: created_at)"
func GetProducts(c *gin.Context) {
	products := []Product{
		{ID: 1, Name: "iPhone 14", Price: 999.99, Description: "Latest iPhone"},
		{ID: 2, Name: "MacBook Pro", Price: 1999.99, Description: "Professional laptop"},
	}
	c.JSON(200, gin.H{"products": products})
}

func CreateProduct(c *gin.Context) {
	var product Product
	c.ShouldBindJSON(&product)
	product.ID = 1
	c.JSON(201, product)
}

// PatchProduct partially updates a product. Accepts a JSON object with any subset of fields.
func PatchProduct(c *gin.Context) {
	id := c.Param("id")
	var updates map[string]interface{}
	if err := c.BindJSON(&updates); err != nil {
		c.JSON(400, gin.H{"error": "invalid request body"})
		return
	}

	product := Product{
		ID:          1,
		Name:        "iPhone 14",
		Price:       999.99,
		Description: "Latest iPhone model",
	}

	if v, ok := updates["name"].(string); ok {
		product.Name = v
	}
	if v, ok := updates["description"].(string); ok {
		product.Description = v
	}
	if v, ok := updates["price"].(float64); ok {
		product.Price = v
	}

	c.JSON(200, gin.H{"id": id, "product": product})
}

// GetProduct retrieves detailed information about a specific product
// @Param id path int true "Product ID to retrieve"
func GetProduct(c *gin.Context) {
	product := Product{
		ID:          1,
		Name:        "iPhone 14",
		Price:       999.99,
		Description: "Latest iPhone model",
	}
	c.JSON(200, product)
}
