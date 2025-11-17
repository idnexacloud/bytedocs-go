package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/idnexacloud/bytedocs-go/pkg/core"
	"github.com/idnexacloud/bytedocs-go/pkg/parser"
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

type APIResponse struct {
	Success bool        `json:"success" example:"true"`
	Message string      `json:"message" example:"Operation completed successfully"`
	Data    interface{} `json:"data"`
}

type ErrorResponse struct {
	Success bool   `json:"success" example:"false"`
	Message string `json:"message" example:"An error occurred"`
	Error   string `json:"error" example:"Resource not found"`
}

var (
	users = []User{
		{ID: 1, Name: "John Doe", Email: "john@example.com"},
		{ID: 2, Name: "Jane Smith", Email: "jane@example.com"},
	}
	products = []Product{
		{ID: 1, Name: "iPhone 14", Price: 999.99, Description: "Latest iPhone model"},
		{ID: 2, Name: "MacBook Pro", Price: 1999.99, Description: "Professional laptop"},
	}
)

func main() {
	config, err := core.LoadConfigFromEnv()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	if err := core.ValidateConfig(config); err != nil {
		log.Fatalf("Invalid config: %v", err)
	}

	if config.Title == "API Documentation" {
		config.Title = "Stdlib API Documentation"
	}
	if config.DocsPath == "" {
		config.DocsPath = "/docs"
	}
	if !config.AutoDetect {
		config.AutoDetect = true
	}


	mux := parser.NewStdlibMuxWrapper()

	parser.SetupStdlibDocs(mux, config)


	mux.HandleFunc("GET /api/v1/products", GetProducts)
	mux.HandleFunc("POST /api/v1/products", CreateProduct)
	mux.HandleFunc("GET /api/v1/products/{id}", GetProduct)
	mux.HandleFunc("PATCH /api/v1/products/{id}", PatchProduct)

	handler := corsMiddleware(mux)

	port := ":8089"
	server := &http.Server{
		Addr:         port,
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	log.Printf("🚀 %s starting on %s", config.Title, port)

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

	log.Println("")

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server failed to start: %v", err)
	}
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func getPathParam(r *http.Request, key string) string {
	return r.PathValue(key)
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message, errorMsg string) {
	writeJSON(w, status, ErrorResponse{
		Success: false,
		Message: message,
		Error:   errorMsg,
	})
}


func GetUsers(w http.ResponseWriter, r *http.Request) {
	response := APIResponse{
		Success: true,
		Message: "Users retrieved successfully",
		Data:    users,
	}
	writeJSON(w, http.StatusOK, response)
}

func CreateUser(w http.ResponseWriter, r *http.Request) {
	var user User
	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	user.ID = len(users) + 1
	users = append(users, user)

	response := APIResponse{
		Success: true,
		Message: "User created successfully",
		Data:    user,
	}
	writeJSON(w, http.StatusCreated, response)
}

func GetUser(w http.ResponseWriter, r *http.Request) {
	id := getPathParam(r, "id")
	userID, err := strconv.Atoi(id)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid user ID", err.Error())
		return
	}

	for _, user := range users {
		if user.ID == userID {
			response := APIResponse{
				Success: true,
				Message: "User retrieved successfully",
				Data:    user,
			}
			writeJSON(w, http.StatusOK, response)
			return
		}
	}

	writeError(w, http.StatusNotFound, "User not found", "User with specified ID does not exist")
}

func UpdateUser(w http.ResponseWriter, r *http.Request) {
	id := getPathParam(r, "id")
	userID, err := strconv.Atoi(id)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid user ID", err.Error())
		return
	}

	var updatedUser User
	if err := json.NewDecoder(r.Body).Decode(&updatedUser); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	for i, user := range users {
		if user.ID == userID {
			updatedUser.ID = userID
			users[i] = updatedUser

			response := APIResponse{
				Success: true,
				Message: "User updated successfully",
				Data:    updatedUser,
			}
			writeJSON(w, http.StatusOK, response)
			return
		}
	}

	writeError(w, http.StatusNotFound, "User not found", "User with specified ID does not exist")
}

func PatchUser(w http.ResponseWriter, r *http.Request) {
	id := getPathParam(r, "id")
	userID, err := strconv.Atoi(id)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid user ID", err.Error())
		return
	}

	var updates map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	for i, user := range users {
		if user.ID == userID {
			if name, ok := updates["name"].(string); ok {
				users[i].Name = name
			}
			if email, ok := updates["email"].(string); ok {
				users[i].Email = email
			}

			response := APIResponse{
				Success: true,
				Message: "User updated successfully",
				Data:    users[i],
			}
			writeJSON(w, http.StatusOK, response)
			return
		}
	}

	writeError(w, http.StatusNotFound, "User not found", "User with specified ID does not exist")
}

func DeleteUser(w http.ResponseWriter, r *http.Request) {
	id := getPathParam(r, "id")
	userID, err := strconv.Atoi(id)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid user ID", err.Error())
		return
	}

	for i, user := range users {
		if user.ID == userID {
			users = append(users[:i], users[i+1:]...)

			response := APIResponse{
				Success: true,
				Message: "User deleted successfully",
				Data: map[string]interface{}{
					"deleted_user_id": userID,
				},
			}
			writeJSON(w, http.StatusOK, response)
			return
		}
	}

	writeError(w, http.StatusNotFound, "User not found", "User with specified ID does not exist")
}


func GetProducts(w http.ResponseWriter, r *http.Request) {
	response := APIResponse{
		Success: true,
		Message: "Products retrieved successfully",
		Data:    products,
	}
	writeJSON(w, http.StatusOK, response)
}

func CreateProduct(w http.ResponseWriter, r *http.Request) {
	var product Product
	if err := json.NewDecoder(r.Body).Decode(&product); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	product.ID = len(products) + 1
	products = append(products, product)

	response := APIResponse{
		Success: true,
		Message: "Product created successfully",
		Data:    product,
	}
	writeJSON(w, http.StatusCreated, response)
}

func GetProduct(w http.ResponseWriter, r *http.Request) {
	id := getPathParam(r, "id")
	productID, err := strconv.Atoi(id)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid product ID", err.Error())
		return
	}

	for _, product := range products {
		if product.ID == productID {
			response := APIResponse{
				Success: true,
				Message: "Product retrieved successfully",
				Data:    product,
			}
			writeJSON(w, http.StatusOK, response)
			return
		}
	}

	writeError(w, http.StatusNotFound, "Product not found", "Product with specified ID does not exist")
}

func PatchProduct(w http.ResponseWriter, r *http.Request) {
	id := getPathParam(r, "id")
	productID, err := strconv.Atoi(id)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid product ID", err.Error())
		return
	}

	var updates map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	for i, product := range products {
		if product.ID == productID {
			if name, ok := updates["name"].(string); ok {
				products[i].Name = name
			}
			if price, ok := updates["price"].(float64); ok {
				products[i].Price = price
			}
			if description, ok := updates["description"].(string); ok {
				products[i].Description = description
			}

			response := APIResponse{
				Success: true,
				Message: "Product updated successfully",
				Data:    products[i],
			}
			writeJSON(w, http.StatusOK, response)
			return
		}
	}

	writeError(w, http.StatusNotFound, "Product not found", "Product with specified ID does not exist")
}