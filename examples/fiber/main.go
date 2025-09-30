package main

import (
	"log"
	"strings"

	"github.com/aibnuhibban/bytedocs/pkg/core"
	_ "github.com/aibnuhibban/bytedocs/pkg/llm"
	"github.com/aibnuhibban/bytedocs/pkg/parser"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
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
		config.Title = "Fiber API Documentation"
	}
	if config.DocsPath == "" {
		config.DocsPath = "/docs"
	}
	if !config.AutoDetect {
		config.AutoDetect = true
	}


	app := fiber.New(fiber.Config{
		AppName:      "Fiber API v1.0.0",
		ServerHeader: "Fiber API",
	})

	app.Use(recover.New())
	app.Use(logger.New(logger.Config{
		Format: "[${time}] ${status} - ${method} ${path} ${latency}\n",
	}))
	app.Use(cors.New(cors.Config{
		AllowOrigins:     "*",
		AllowMethods:     "GET,POST,PUT,PATCH,DELETE,OPTIONS",
		AllowHeaders:     "Origin,Content-Type,Accept,Authorization",
		AllowCredentials: false,
	}))

	parser.SetupFiberDocs(app, config)

	api := app.Group("/api/v1")
	{
		users := api.Group("/users")
		{
			users.Get("", GetUsers)
			users.Post("", CreateUser)
			users.Get("/:id", GetUser)
			users.Put("/:id", UpdateUser)

			users.Patch("/:id", PatchUser)
			users.Delete("/:id", DeleteUser)
		}

		products := api.Group("/products")
		{
			products.Get("", GetProducts)
			products.Post("", CreateProduct)
			products.Get("/:id", GetProduct)

			products.Patch("/:id", PatchProduct)
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

	log.Fatal(app.Listen(port))
}

// GetUsers retrieves a list of all users with pagination support
// @Param page query int false "Page number for pagination (default: 1)"
// @Param limit query int false "Number of users per page (default: 10, max: 100)"
// @Param search query string false "Search term to filter users by name or email"
func GetUsers(c *fiber.Ctx) error {
	users := []User{
		{ID: 1, Name: "John Doe", Email: "john@example.com"},
		{ID: 2, Name: "Jane Smith", Email: "jane@example.com"},
	}
	return c.JSON(map[string]interface{}{"users": users})
}

func CreateUser(c *fiber.Ctx) error {
	var user User
	if err := c.BodyParser(&user); err != nil {
		return c.Status(400).JSON(map[string]string{"error": "invalid request body"})
	}
	user.ID = 123
	return c.Status(201).JSON(user)
}

// GetUser retrieves detailed information about a specific user
// @Param id path int true "User ID to retrieve"
func GetUser(c *fiber.Ctx) error {
	_ = c.Params("id")
	user := User{
		ID:    123,
		Name:  "John Doe",
		Email: "john@example.com",
	}
	return c.JSON(user)
}

// UpdateUser updates user information (requires authentication)
// @Param id path int true "User ID to update"
func UpdateUser(c *fiber.Ctx) error {
	var user User
	if err := c.BodyParser(&user); err != nil {
		return c.Status(400).JSON(map[string]string{"error": "invalid request body"})
	}
	return c.JSON(user)
}

// PatchUser partially updates a user. Accepts a JSON object with any subset of fields.
func PatchUser(c *fiber.Ctx) error {
	id := c.Params("id")
	var updates map[string]interface{}
	if err := c.BodyParser(&updates); err != nil {
		return c.Status(400).JSON(map[string]string{"error": "invalid request body"})
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

	return c.JSON(map[string]interface{}{"id": id, "user": user})
}

// DeleteUser permanently deletes a user account
// @Param id path int true "User ID to delete"
func DeleteUser(c *fiber.Ctx) error {
	return c.Status(204).JSON(nil)
}

// GetProducts retrieves all products with filtering options
// @Param category query string false "Filter products by category (e.g., Electronics, Clothing)"
// @Param min_price query number false "Minimum price filter"
// @Param max_price query number false "Maximum price filter"
// @Param sort query string false "Sort by: name, price, created_at (default: created_at)"
func GetProducts(c *fiber.Ctx) error {
	products := []Product{
		{ID: 1, Name: "iPhone 14", Price: 999.99, Description: "Latest iPhone"},
		{ID: 2, Name: "MacBook Pro", Price: 1999.99, Description: "Professional laptop"},
	}
	return c.JSON(map[string]interface{}{"products": products})
}

func CreateProduct(c *fiber.Ctx) error {
	var product Product
	if err := c.BodyParser(&product); err != nil {
		return c.Status(400).JSON(map[string]string{"error": "invalid request body"})
	}
	product.ID = 1
	return c.Status(201).JSON(product)
}

// PatchProduct partially updates a product. Accepts a JSON object with any subset of fields.
func PatchProduct(c *fiber.Ctx) error {
	id := c.Params("id")
	var updates map[string]interface{}
	if err := c.BodyParser(&updates); err != nil {
		return c.Status(400).JSON(map[string]string{"error": "invalid request body"})
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

	return c.JSON(map[string]interface{}{"id": id, "product": product})
}

// GetProduct retrieves detailed information about a specific product
// @Param id path int true "Product ID to retrieve"
func GetProduct(c *fiber.Ctx) error {
	product := Product{
		ID:          1,
		Name:        "iPhone 14",
		Price:       999.99,
		Description: "Latest iPhone model",
	}
	return c.JSON(product)
}