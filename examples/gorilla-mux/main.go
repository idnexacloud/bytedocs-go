package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/aibnuhibban/bytedocs/pkg/core"
	_ "github.com/aibnuhibban/bytedocs/pkg/llm"
	"github.com/aibnuhibban/bytedocs/pkg/parser"
	"github.com/gorilla/mux"
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
		config.Title = "Gorilla-Mux API Documentation"
	}
	if config.DocsPath == "" {
		config.DocsPath = "/docs"
	}
	if !config.AutoDetect {
		config.AutoDetect = true
	}


	r := parser.NewGorillaMuxWrapper()

	r.Use(corsMiddleware)

	parser.SetupGorillaMuxDocs(r, config)

	api := r.PathPrefix("/api/v1").Subrouter()

	users := api.PathPrefix("/users").Subrouter()
	users.HandleFunc("", GetUsers).Methods("GET")
	users.HandleFunc("", CreateUser).Methods("POST")
	users.HandleFunc("/{id:[0-9]+}", GetUser).Methods("GET")
	users.HandleFunc("/{id:[0-9]+}", UpdateUser).Methods("PUT")
	
	users.HandleFunc("/{id:[0-9]+}", PatchUser).Methods("PATCH")
	users.HandleFunc("/{id:[0-9]+}", DeleteUser).Methods("DELETE")

	products := api.PathPrefix("/products").Subrouter()
	products.HandleFunc("", GetProducts).Methods("GET")
	products.HandleFunc("", CreateProduct).Methods("POST")
	products.HandleFunc("/{id:[0-9]+}", GetProduct).Methods("GET")
	
	products.HandleFunc("/{id:[0-9]+}", PatchProduct).Methods("PATCH")

	port := ":8080"
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

	srv := &http.Server{
		Addr:         port,
		Handler:      r,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization, X-API-Key")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func getIntParam(r *http.Request, key string) (int, error) {
	value := mux.Vars(r)[key]
	return strconv.Atoi(value)
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// GetUsers retrieves a list of all users with pagination support
// @Param page query int false "Page number for pagination (default: 1)"
// @Param limit query int false "Number of users per page (default: 10, max: 100)"
// @Param search query string false "Search term to filter users by name or email"
func GetUsers(w http.ResponseWriter, r *http.Request) {
	users := []User{
		{ID: 1, Name: "John Doe", Email: "john@example.com"},
		{ID: 2, Name: "Jane Smith", Email: "jane@example.com"},
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"users": users})
}

func CreateUser(w http.ResponseWriter, r *http.Request) {
	var user User
	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	user.ID = 123
	writeJSON(w, http.StatusCreated, user)
}

// GetUser retrieves detailed information about a specific user
// @Param id path int true "User ID to retrieve"
func GetUser(w http.ResponseWriter, r *http.Request) {
	_ = mux.Vars(r)["id"]
	user := User{
		ID:    123,
		Name:  "John Doe",
		Email: "john@example.com",
	}
	writeJSON(w, http.StatusOK, user)
}

// UpdateUser updates user information (requires authentication)
// @Param id path int true "User ID to update"
func UpdateUser(w http.ResponseWriter, r *http.Request) {
	var user User
	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	writeJSON(w, http.StatusOK, user)
}

// PatchUser partially updates a user. Accepts a JSON object with any subset of fields.
func PatchUser(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	var updates map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
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

	writeJSON(w, http.StatusOK, map[string]interface{}{"id": id, "user": user})
}

// DeleteUser permanently deletes a user account
// @Param id path int true "User ID to delete"
func DeleteUser(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusNoContent, nil)
}

// GetProducts retrieves all products with filtering options
// @Param category query string false "Filter products by category (e.g., Electronics, Clothing)"
// @Param min_price query number false "Minimum price filter"
// @Param max_price query number false "Maximum price filter"
// @Param sort query string false "Sort by: name, price, created_at (default: created_at)"
func GetProducts(w http.ResponseWriter, r *http.Request) {
	products := []Product{
		{ID: 1, Name: "iPhone 14", Price: 999.99, Description: "Latest iPhone"},
		{ID: 2, Name: "MacBook Pro", Price: 1999.99, Description: "Professional laptop"},
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"products": products})
}

func CreateProduct(w http.ResponseWriter, r *http.Request) {
	var product Product
	if err := json.NewDecoder(r.Body).Decode(&product); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	product.ID = 1
	writeJSON(w, http.StatusCreated, product)
}

// PatchProduct partially updates a product. Accepts a JSON object with any subset of fields.
func PatchProduct(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	var updates map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
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

	writeJSON(w, http.StatusOK, map[string]interface{}{"id": id, "product": product})
}

// GetProduct retrieves detailed information about a specific product
// @Param id path int true "Product ID to retrieve"
func GetProduct(w http.ResponseWriter, r *http.Request) {
	product := Product{
		ID:          1,
		Name:        "iPhone 14",
		Price:       999.99,
		Description: "Latest iPhone model",
	}
	writeJSON(w, http.StatusOK, product)
}