package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/aibnuhibban/bytedocs/pkg/core"
	"github.com/aibnuhibban/bytedocs/pkg/parser"
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
		config.Title = "Net/HTTP API Documentation"
	}
	if config.DocsPath == "" {
		config.DocsPath = "/docs"
	}
	if !config.AutoDetect {
		config.AutoDetect = true
	}


	mux := parser.NewNetHTTPMuxWrapper()

	parser.SetupNetHTTPDocs(mux, config)

	mux.HandleFunc("GET /api/v1/users", GetUsers)
	mux.HandleFunc("POST /api/v1/users", CreateUser)
	mux.HandleFunc("GET /api/v1/users/{id}", GetUser)
	mux.HandleFunc("PUT /api/v1/users/{id}", UpdateUser)
	mux.HandleFunc("PATCH /api/v1/users/{id}", PatchUser)
	mux.HandleFunc("DELETE /api/v1/users/{id}", DeleteUser)

	mux.HandleFunc("GET /api/v1/products", GetProducts)
	mux.HandleFunc("POST /api/v1/products", CreateProduct)
	mux.HandleFunc("GET /api/v1/products/{id}", GetProduct)
	mux.HandleFunc("PATCH /api/v1/products/{id}", PatchProduct)

	server := &http.Server{
		Addr:         ":8080",
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	log.Println("🚀 Net/HTTP API Documentation starting on :8080")
	log.Println("")

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server failed to start: %v", err)
	}
}

func getPathParam(r *http.Request, key string) string {
	return r.PathValue(key)
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

// CreateUser creates a new user
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
func GetUser(w http.ResponseWriter, r *http.Request) {
	_ = getPathParam(r, "id")
	user := User{
		ID:    123,
		Name:  "John Doe",
		Email: "john@example.com",
	}
	writeJSON(w, http.StatusOK, user)
}

// UpdateUser updates user information (requires authentication)
func UpdateUser(w http.ResponseWriter, r *http.Request) {
	var user User
	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	id := getPathParam(r, "id")
	userID, _ := strconv.Atoi(id)
	user.ID = userID
	writeJSON(w, http.StatusOK, user)
}

// PatchUser partially updates a user. Accepts a JSON object with any subset of fields.
func PatchUser(w http.ResponseWriter, r *http.Request) {
	var updates map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	id := getPathParam(r, "id")
	user := User{
		ID:    123,
		Name:  "John Doe",
		Email: "john@example.com",
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"id":   id,
		"user": user,
	})
}

// DeleteUser permanently deletes a user account
func DeleteUser(w http.ResponseWriter, r *http.Request) {
	_ = getPathParam(r, "id")
	writeJSON(w, http.StatusNoContent, "")
}

// GetProducts retrieves all products with filtering options
// @Param category query string false "Filter products by category"
// @Param min_price query float false "Minimum price filter"
// @Param max_price query float false "Maximum price filter"
// @Param sort query string false "Sort order (price_asc, price_desc, name_asc, name_desc)"
func GetProducts(w http.ResponseWriter, r *http.Request) {
	products := []Product{
		{ID: 1, Name: "iPhone 14", Price: 999.99, Description: "Latest iPhone"},
		{ID: 2, Name: "MacBook Pro", Price: 1999.99, Description: "Professional laptop"},
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"products": products})
}

// CreateProduct creates a new product
func CreateProduct(w http.ResponseWriter, r *http.Request) {
	var product Product
	if err := json.NewDecoder(r.Body).Decode(&product); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	product.ID = 123
	writeJSON(w, http.StatusCreated, product)
}

// GetProduct retrieves detailed information about a specific product
func GetProduct(w http.ResponseWriter, r *http.Request) {
	_ = getPathParam(r, "id")
	product := Product{
		ID:          1,
		Name:        "iPhone 14",
		Price:       999.99,
		Description: "Latest iPhone model",
	}
	writeJSON(w, http.StatusOK, product)
}

// PatchProduct partially updates a product. Accepts a JSON object with any subset of fields.
func PatchProduct(w http.ResponseWriter, r *http.Request) {
	var updates map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	id := getPathParam(r, "id")
	product := Product{
		ID:          1,
		Name:        "iPhone 14",
		Price:       999.99,
		Description: "Latest iPhone model",
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"id":      id,
		"product": product,
	})
}