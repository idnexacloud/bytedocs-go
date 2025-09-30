package main

import (
	"log"
	"net/http"
	"strings"

	"github.com/aibnuhibban/bytedocs/pkg/core"
	_ "github.com/aibnuhibban/bytedocs/pkg/llm"
	"github.com/aibnuhibban/bytedocs/pkg/parser"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
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
	config, err := core.LoadConfigFromEnv()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	if err := core.ValidateConfig(config); err != nil {
		log.Fatalf("Invalid config: %v", err)
	}

	if config.Title == "API Documentation" {
		config.Title = "Echo API Documentation"
	}
	if config.DocsPath == "" {
		config.DocsPath = "/docs"
	}
	if !config.AutoDetect {
		config.AutoDetect = true
	}


	e := echo.New()

	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())

	parser.SetupEchoDocs(e, config)

	api := e.Group("/api/v1")
	{
		users := api.Group("/users")
		{
			users.GET("", GetUsers)
			users.POST("", CreateUser)
			users.GET("/:id", GetUser)
			users.PUT("/:id", UpdateUser)

			users.PATCH("/:id", PatchUser)
			users.DELETE("/:id", DeleteUser)
		}

		products := api.Group("/products")
		{
			products.GET("", GetProducts)
			products.POST("", CreateProduct)
			products.GET("/:id", GetProduct)
			
			products.PATCH("/:id", PatchProduct)
		}
	}

	port := ":1323"
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

	e.Logger.Fatal(e.Start(port))
}

// GetUsers retrieves a list of all users with pagination support
// @Param page query int false "Page number for pagination (default: 1)"
// @Param limit query int false "Number of users per page (default: 10, max: 100)"
// @Param search query string false "Search term to filter users by name or email"
func GetUsers(c echo.Context) error {
	users := []User{
		{ID: 1, Name: "John Doe", Email: "john@example.com"},
		{ID: 2, Name: "Jane Smith", Email: "jane@example.com"},
	}
	return c.JSON(http.StatusOK, map[string]interface{}{"users": users})
}

func CreateUser(c echo.Context) error {
	var user User
	if err := c.Bind(&user); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}
	user.ID = 123
	return c.JSON(http.StatusCreated, user)
}

// GetUser retrieves detailed information about a specific user
// @Param id path int true "User ID to retrieve"
func GetUser(c echo.Context) error {
	_ = c.Param("id")
	user := User{
		ID:    123,
		Name:  "John Doe",
		Email: "john@example.com",
	}
	return c.JSON(http.StatusOK, user)
}

// UpdateUser updates user information (requires authentication)
// @Param id path int true "User ID to update"
func UpdateUser(c echo.Context) error {
	var user User
	if err := c.Bind(&user); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}
	return c.JSON(http.StatusOK, user)
}

// PatchUser partially updates a user. Accepts a JSON object with any subset of fields.
func PatchUser(c echo.Context) error {
	id := c.Param("id")
	var updates map[string]interface{}
	if err := c.Bind(&updates); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}

	// In a real app you'd fetch the user from DB and apply updates.
	// Here we simulate by returning the merged object.
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

	return c.JSON(http.StatusOK, map[string]interface{}{"id": id, "user": user})
}

// DeleteUser permanently deletes a user account
// @Param id path int true "User ID to delete"
func DeleteUser(c echo.Context) error {
	return c.JSON(http.StatusNoContent, nil)
}

// GetProducts retrieves all products with filtering options
// @Param category query string false "Filter products by category (e.g., Electronics, Clothing)"
// @Param min_price query number false "Minimum price filter"
// @Param max_price query number false "Maximum price filter"
// @Param sort query string false "Sort by: name, price, created_at (default: created_at)"
func GetProducts(c echo.Context) error {
	products := []Product{
		{ID: 1, Name: "iPhone 14", Price: 999.99, Description: "Latest iPhone"},
		{ID: 2, Name: "MacBook Pro", Price: 1999.99, Description: "Professional laptop"},
	}
	return c.JSON(http.StatusOK, map[string]interface{}{"products": products})
}

func CreateProduct(c echo.Context) error {
	var product Product
	if err := c.Bind(&product); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}
	product.ID = 1
	return c.JSON(http.StatusCreated, product)
}

// PatchProduct partially updates a product. Accepts a JSON object with any subset of fields.
func PatchProduct(c echo.Context) error {
	id := c.Param("id")
	var updates map[string]interface{}
	if err := c.Bind(&updates); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
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

	return c.JSON(http.StatusOK, map[string]interface{}{"id": id, "product": product})
}

// GetProduct retrieves detailed information about a specific product
// @Param id path int true "Product ID to retrieve"
func GetProduct(c echo.Context) error {
	product := Product{
		ID:          1,
		Name:        "iPhone 14",
		Price:       999.99,
		Description: "Latest iPhone model",
	}
	return c.JSON(http.StatusOK, product)
}