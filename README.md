[![Go Version](https://img.shields.io/badge/Go-1.23.5-blue.svg?style=flat-square)](https://golang.org/)
[![GoDoc](http://img.shields.io/badge/go-documentation-blue.svg?style=flat-square)](https://pkg.go.dev/github.com/thanhthanh221/msa-core)
[![License](http://img.shields.io/badge/license-MIT-blue.svg?style=flat-square)](https://raw.githubusercontent.com/thanhthanh221/msa-core/master/LICENSE)

# MSA Core

High performance, extensible, minimalist core library for Microservices Architecture (MSA) projects in Go, built on top of **Echo framework**.

## Overview

MSA Core is a comprehensive core module designed to provide essential infrastructure components, utilities, and middleware for building robust microservices applications with **Echo framework**. It serves as a shared foundation that other projects can import and use, providing a complete set of tools for building RESTful APIs and microservices.

## Features

- 🚀 **Infrastructure Components**
  - **Redis** (`pkg/infrastructure/redis`): High-performance Redis client with OpenTelemetry tracing support, cluster mode, and comprehensive operations
  - **RabbitMQ** (`pkg/infrastructure/rabbitmq`): Message queue client with DLX/DLQ support, distributed tracing, and full exchange/queue management
  - **GORM Repository** (`pkg/infrastructure/repositories`): Generic repository pattern with GORM, OpenTelemetry tracing support, transaction management, and comprehensive CRUD operations
  - **MinIO** (`pkg/infrastructure/minio`): S3-compatible object storage client with OpenTelemetry tracing
  - **Cloudinary** (`pkg/infrastructure/cloudinary`): Media management and transformation service

- 🔧 **Echo Framework Support**
  - **Complete Echo Integration**: Full support for Echo v4 framework with comprehensive middleware and helpers
  - **Tracing Middleware** (`pkg/middleware`): Distributed tracing with OpenTelemetry for HTTP requests (write operations)
  - **Response Handler** (`pkg/middleware`): Automatic response wrapping in standardized format with i18n support and processing time tracking
  - **Error Handler** (`pkg/middleware`): Centralized error handling with proper HTTP status codes and error messages
  - **Validation Error Handler** (`pkg/middleware`): Specialized handler for validation errors
  - **JWT Auth Middleware** (`pkg/middleware`): JWT token authentication and authorization with scope-based access control
  - **API Key Auth** (`pkg/middleware`): API key-based authentication middleware
  - **Response Helper** (`pkg/helpers`): Helper functions for standardized API responses (Success, Error, ValidationError, etc.)
  - **Request Helper** (`pkg/helpers`): Utility functions for extracting trace IDs and other request information
  - **JWT Helper** (`pkg/helpers`): Helper functions for JWT token verification in Echo context

- 📦 **Common Utilities**
  - **Base Response** (`pkg/common`): Standardized API response structure with error handling
  - **I18n** (`pkg/common`): Internationalization support with locale management
  - **Context Keys** (`pkg/common`): Context key definitions for request context

- 🎯 **Services**
  - **JWT Service** (`pkg/service`): JWT token generation, validation, and management

- 📋 **Models**
  - **Auth Models** (`pkg/models`): Authentication and authorization data models
  - **RabbitMQ Models** (`pkg/models`): RabbitMQ configuration and option models

- 🛠️ **Utilities**
  - **Errors** (`pkg/utils`): Custom error types and error handling utilities
  - **Utils** (`pkg/utils`): JSON utilities and helper functions

- 📌 **Version**
  - **Version** (`pkg/version`): Application version management and retrieval

- 🏗️ **Architecture & Observability**
  - Clean architecture with separated concerns
  - Interface-based design for easy testing
  - **OpenTelemetry Integration**: Full support for distributed tracing with OpenTelemetry
  - **Jaeger Tracing Support**: Export traces to Jaeger for visualization and analysis
  - Structured logging support with Logrus

## Installation

```bash
go get github.com/thanhthanh221/msa-core
```

### Requirements

- Go 1.23.5 or higher

## Quick Start

### Basic Usage

```go
package main

import (
    "context"
    "fmt"
    
    "github.com/thanhthanh221/msa-core/pkg/utils"
    "github.com/thanhthanh221/msa-core/pkg/version"
    "github.com/thanhthanh221/msa-core/internal/logger"
)

func main() {
    // Logger
    log := logger.NewStdLogger()
    log.Info("Application started")
    
    // Version
    fmt.Printf("MSA Core Version: %s\n", version.GetVersion())
    
    // Error handling
    err := utils.NewAppError("ERR001", "Example error", nil)
    log.Error(err.Error())
}
```

### Using Redis Client

```go
package main

import (
    "context"
    "fmt"
    "time"
    
    "github.com/thanhthanh221/msa-core/pkg/infrastructure/redis"
    "go.opentelemetry.io/otel/trace"
)

func main() {
    // Initialize with cluster mode (optional), address, password, prefix, and tracer
    clusterEnv := ""  // e.g., "localhost:7001,localhost:7002,localhost:7003"
    address := "localhost:6379"
    password := ""
    prefix := "myapp"
    tracer := trace.NewNoopTracerProvider()  // Or use your actual tracer
    
    client := redis.NewRedisClient(clusterEnv, address, password, prefix, tracer)
    defer client.Close()
    
    ctx := context.Background()
    
    // Set value
    err := client.Set(ctx, "key", "value", time.Hour)
    if err != nil {
        panic(err)
    }
    
    // Get value
    value, err := client.Get(ctx, "key")
    if err != nil {
        panic(err)
    }
    
    fmt.Println("Value:", value)
}
```

### Using RabbitMQ Client

```go
package main

import (
    "context"
    "fmt"
    "log"
    
    amqp "github.com/rabbitmq/amqp091-go"
    "github.com/sirupsen/logrus"
    "go.opentelemetry.io/otel/trace"
    
    "github.com/thanhthanh221/msa-core/pkg/infrastructure/rabbitmq"
    "github.com/thanhthanh221/msa-core/pkg/models"
)

func main() {
    logger := logrus.New()
    tracer := trace.NewNoopTracerProvider()
    
    // Initialize RabbitMQ client
    client, err := rabbitmq.NewRabbitMQClient("amqp://guest:guest@localhost:5672/", logger, tracer)
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()
    
    ctx := context.Background()
    
    // Setup infrastructure (declare exchanges, queues, bindings)
    // Declare exchange
    err = client.DeclareExchange(ctx, "order-exchange", "topic", true, false, false, false, nil)
    if err != nil {
        log.Fatal(err)
    }
    
    // Declare queue with DLX support
    queueOptions := models.QueueOptions{
        Durable:    true,
        DLXName:    "order-dlx",
        DLXRoutingKey: "order-dlq",
        MaxRetries: 3,
    }
    err = client.DeclareQueueWithDLX(ctx, "order-queue", queueOptions)
    if err != nil {
        log.Fatal(err)
    }
    
    // Bind queue to exchange
    err = client.BindQueue(ctx, "order-queue", "order.*", "order-exchange", false, nil)
    if err != nil {
        log.Fatal(err)
    }
    
    // Publish message
    message := map[string]interface{}{
        "orderId": "12345",
        "amount":  100.50,
    }
    err = client.Publish(ctx, "order-exchange", "order.created", message)
    if err != nil {
        log.Fatal(err)
    }
    
    // Consume messages
    err = client.Consume(ctx, "order-queue", func(ctx context.Context, delivery amqp.Delivery) error {
        fmt.Printf("Received message: %s\n", delivery.Body)
        // Process message...
        return nil // Return error to reject message
    })
    if err != nil {
        log.Fatal(err)
    }
}
```

### Using GORM Repository

```go
package main

import (
    "context"
    "fmt"
    "log"
    
    "github.com/sirupsen/logrus"
    "go.opentelemetry.io/otel/trace"
    "gorm.io/driver/mysql"
    "gorm.io/gorm"
    
    "github.com/thanhthanh221/msa-core/pkg/infrastructure/repositories"
)

type User struct {
    ID    uint   `gorm:"primaryKey"`
    Name  string
    Email string
}

func main() {
    logger := logrus.New()
    tracer := trace.NewNoopTracerProvider()
    
    // Initialize GORM database connection
    dsn := "user:password@tcp(localhost:3306)/dbname?charset=utf8mb4&parseTime=True&loc=Local"
    db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
    if err != nil {
        log.Fatal(err)
    }
    
    // Create repository with tracing support
    repo := repositories.NewGormRepository(db, logger, tracer)
    
    ctx := context.Background()
    
    // Create a new user
    user := &User{
        Name:  "John Doe",
        Email: "john@example.com",
    }
    if err := repo.Create(ctx, user); err != nil {
        log.Fatal(err)
    }
    
    // Get user by ID (with tracing)
    var foundUser User
    if err := repo.GetOneByID(ctx, &foundUser, user.ID); err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Found user: %+v\n", foundUser)
    
    // Get users by field (with tracing)
    var users []User
    if err := repo.GetByField(ctx, &users, "email", "john@example.com"); err != nil {
        log.Fatal(err)
    }
    
    // Update user
    updates := map[string]interface{}{
        "name": "Jane Doe",
    }
    if err := repo.Update(ctx, &User{}, updates, "id = ?", user.ID); err != nil {
        log.Fatal(err)
    }
    
    // Transaction example (with tracing)
    tx, err := repo.BeginTx(ctx)
    if err != nil {
        log.Fatal(err)
    }
    
    newUser := &User{Name: "Transaction User", Email: "tx@example.com"}
    if err := repo.CreateTx(ctx, newUser, tx); err != nil {
        tx.Rollback()
        log.Fatal(err)
    }
    
    if err := repo.CommitTx(ctx, tx); err != nil {
        log.Fatal(err)
    }
}
```

### Building RESTful APIs with Echo

MSA Core provides comprehensive support for building RESTful APIs with Echo framework:

```go
package main

import (
    "github.com/labstack/echo/v4"
    "github.com/sirupsen/logrus"
    "go.opentelemetry.io/otel/trace"
    
    "github.com/thanhthanh221/msa-core/pkg/helpers"
    "github.com/thanhthanh221/msa-core/pkg/middleware"
)

func main() {
    e := echo.New()
    logrusLogger := logrus.New()
    tracerProvider := trace.NewNoopTracerProvider()
    
    // 1. Tracing middleware (creates spans for write operations)
    tracingMiddleware := middleware.NewTracingMiddleware(tracerProvider, logrusLogger)
    e.Use(tracingMiddleware.Middleware())
    
    // 2. Response handler middleware (automatically wraps responses)
    e.Use(middleware.ResponseHandlerMiddleware())
    
    // 3. Error handler middleware (centralized error handling)
    e.Use(middleware.ErrorHandlerMiddleware())
    
    // 4. Validation error handler (for validation errors)
    e.Use(middleware.ValidationErrorHandler())
    
    // Public routes
    e.GET("/health", func(c echo.Context) error {
        return c.JSON(200, map[string]string{"status": "ok"})
    })
    
    // Protected routes with JWT authentication
    jwtMiddleware := middleware.NewJWTAuthMiddleware("your-secret-key", logrusLogger)
    protected := e.Group("/api")
    protected.Use(jwtMiddleware.RequireAuth())
    
    // Protected route with scope requirement
    admin := protected.Group("/admin")
    admin.Use(jwtMiddleware.RequireScope("admin"))
    
    // Example: Using Response Helper
    responseHelper := helpers.NewResponseHelper()
    protected.GET("/users", func(c echo.Context) error {
        // Your business logic here
        users := []map[string]interface{}{
            {"id": 1, "name": "John"},
            {"id": 2, "name": "Jane"},
        }
        return responseHelper.Success(c, users, "Users retrieved successfully")
    })
    
    // Example: Using middleware.SetResponseData
    protected.GET("/products", func(c echo.Context) error {
        products := []map[string]interface{}{
            {"id": 1, "name": "Product 1"},
        }
        middleware.SetResponseData(c, products, "Products retrieved successfully")
        return nil // ResponseHandlerMiddleware will wrap it automatically
    })
    
    // Example: Error handling
    protected.GET("/users/:id", func(c echo.Context) error {
        id := c.Param("id")
        if id == "" {
            return responseHelper.BadRequest(c, "User ID is required")
        }
        
        // Your business logic
        user := map[string]interface{}{"id": id, "name": "John"}
        if user == nil {
            return responseHelper.NotFound(c, "User not found")
        }
        
        return responseHelper.Success(c, user, "User retrieved successfully")
    })
    
    // API Key protected route
    apiKeyGroup := e.Group("/api/v1")
    apiKeyGroup.Use(middleware.APIKeyAuthMiddleware("your-api-key"))
    apiKeyGroup.GET("/data", func(c echo.Context) error {
        data := map[string]string{"message": "Protected data"}
        return responseHelper.Success(c, data, "Data retrieved successfully")
    })
    
    e.Logger.Fatal(e.Start(":8080"))
}
```

### Complete Echo API Example with Repository

```go
package main

import (
    "context"
    "net/http"
    
    "github.com/labstack/echo/v4"
    "github.com/sirupsen/logrus"
    "go.opentelemetry.io/otel/trace"
    "gorm.io/driver/mysql"
    "gorm.io/gorm"
    
    "github.com/thanhthanh221/msa-core/pkg/helpers"
    "github.com/thanhthanh221/msa-core/pkg/infrastructure/repositories"
    "github.com/thanhthanh221/msa-core/pkg/middleware"
)

type User struct {
    ID    uint   `gorm:"primaryKey" json:"id"`
    Name  string `json:"name"`
    Email string `json:"email"`
}

type UserHandler struct {
    repo           repositories.TransactionRepository
    responseHelper *helpers.ResponseHelper
}

func NewUserHandler(repo repositories.TransactionRepository) *UserHandler {
    return &UserHandler{
        repo:           repo,
        responseHelper: helpers.NewResponseHelper(),
    }
}

func (h *UserHandler) GetUsers(c echo.Context) error {
    ctx := c.Request().Context()
    var users []User
    
    if err := h.repo.GetAll(ctx, &users); err != nil {
        return h.responseHelper.InternalError(c, "Failed to retrieve users")
    }
    
    return h.responseHelper.Success(c, users, "Users retrieved successfully")
}

func (h *UserHandler) GetUser(c echo.Context) error {
    ctx := c.Request().Context()
    id := c.Param("id")
    
    var user User
    if err := h.repo.GetOneByID(ctx, &user, id); err != nil {
        if err == repositories.ErrNotFound {
            return h.responseHelper.NotFound(c, "User not found")
        }
        return h.responseHelper.InternalError(c, "Failed to retrieve user")
    }
    
    return h.responseHelper.Success(c, user, "User retrieved successfully")
}

func (h *UserHandler) CreateUser(c echo.Context) error {
    ctx := c.Request().Context()
    var user User
    
    if err := c.Bind(&user); err != nil {
        return h.responseHelper.BadRequest(c, "Invalid request body")
    }
    
    if err := h.repo.Create(ctx, &user); err != nil {
        return h.responseHelper.InternalError(c, "Failed to create user")
    }
    
    return h.responseHelper.Success(c, user, "User created successfully")
}

func main() {
    e := echo.New()
    logger := logrus.New()
    tracer := trace.NewNoopTracerProvider()
    
    // Setup database
    dsn := "user:password@tcp(localhost:3306)/dbname?charset=utf8mb4&parseTime=True&loc=Local"
    db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
    if err != nil {
        logger.Fatal(err)
    }
    
    // Create repository
    repo := repositories.NewGormRepository(db, logger, tracer)
    
    // Setup middleware
    tracingMiddleware := middleware.NewTracingMiddleware(tracer, logger)
    e.Use(tracingMiddleware.Middleware())
    e.Use(middleware.ResponseHandlerMiddleware())
    e.Use(middleware.ErrorHandlerMiddleware())
    
    // Setup handlers
    userHandler := NewUserHandler(repo)
    
    // Routes
    api := e.Group("/api")
    api.GET("/users", userHandler.GetUsers)
    api.GET("/users/:id", userHandler.GetUser)
    api.POST("/users", userHandler.CreateUser)
    
    e.Logger.Fatal(e.Start(":8080"))
}
```

## Project Structure

```
msa-core/
├── pkg/
│   ├── common/              # Common utilities
│   │   ├── base_response.go    # Standardized response structure
│   │   ├── i18n.go             # Internationalization manager
│   │   ├── i18n_helper.go      # I18n helper functions
│   │   └── context_keys.go     # Context key definitions
│   ├── infrastructure/      # Infrastructure adapters
│   │   ├── redis/             # Redis client with tracing
│   │   │   └── redis.go
│   │   ├── rabbitmq/          # RabbitMQ client with DLX/DLQ
│   │   │   └── rabbitmq.go
│   │   ├── repositories/      # GORM repository pattern with tracing
│   │   │   ├── repository.go
│   │   │   ├── types.go
│   │   │   └── errors.go
│   │   ├── minio/             # MinIO (S3) client
│   │   │   └── minio.go
│   │   └── cloudinary/        # Cloudinary client
│   │       └── cloudinary.go
│   ├── middleware/          # Echo HTTP middlewares
│   │   ├── tracing_middleware.go    # OpenTelemetry tracing
│   │   ├── response_handler.go      # Response standardization
│   │   ├── jwt_auth_middleware.go    # JWT authentication
│   │   └── api_key_auth.go          # API key authentication
│   ├── helpers/            # Echo helper functions
│   │   ├── response_helper.go       # Response helpers
│   │   ├── request_helper.go        # Request helpers
│   │   ├── jwt_helper.go           # JWT helpers
│   │   └── ...                     # Other helpers
│   ├── service/             # Business services
│   │   └── jwt_service.go       # JWT token service
│   ├── models/              # Data models
│   │   ├── auth_models.go       # Authentication models
│   │   └── rabbitmq.go          # RabbitMQ models
│   ├── utils/               # Utility functions
│   │   ├── errors.go           # Error handling
│   │   ├── errors_test.go      # Error tests
│   │   └── utils.go            # JSON and other utilities
│   └── version/             # Version information
│       ├── version.go
│       └── version_test.go
├── internal/
│   └── logger/              # Logger implementation
│       └── logger.go
├── examples/                # Example code
└── LICENSE
```

## Packages

### Infrastructure

- **Redis** (`pkg/infrastructure/redis`): High-performance Redis client with OpenTelemetry tracing support, cluster mode, and comprehensive operations (Set, Get, Hash operations, etc.)
- **RabbitMQ** (`pkg/infrastructure/rabbitmq`): Message queue client with Dead Letter Exchange (DLX) and Dead Letter Queue (DLQ) support, distributed tracing, and full exchange/queue/binding management
- **GORM Repository** (`pkg/infrastructure/repositories`): Generic repository pattern implementation with GORM, featuring:
  - OpenTelemetry distributed tracing for all database operations
  - Transaction support with BeginTx, CommitTx, Rollback
  - Comprehensive CRUD operations (Create, Read, Update, Delete)
  - Advanced querying (GetByField, GetByFields, GetWhere, pagination)
  - Preload and Joins support for eager loading
  - Count operations with filters
  - Raw SQL query support
  - Error handling with proper error types
  - Support for multiple database types (MySQL, PostgreSQL, SQLite)
- **MinIO** (`pkg/infrastructure/minio`): S3-compatible object storage client with OpenTelemetry tracing
- **Cloudinary** (`pkg/infrastructure/cloudinary`): Media management and transformation service

### Echo Framework Support

- **Tracing Middleware** (`pkg/middleware`): Distributed tracing with OpenTelemetry for HTTP requests (write operations). Automatically creates spans for POST, PUT, DELETE, PATCH requests. Supports Jaeger export for trace visualization. Supports Jaeger export for trace visualization
- **Response Handler** (`pkg/middleware`): Automatic response wrapping in standardized format with i18n support and processing time tracking. Automatically wraps successful responses in base response structure
- **Error Handler** (`pkg/middleware`): Centralized error handling middleware that converts errors to proper HTTP responses with appropriate status codes
- **Validation Error Handler** (`pkg/middleware`): Specialized error handler for validation errors with detailed error information
- **JWT Auth Middleware** (`pkg/middleware`): JWT token authentication and authorization with:
  - Token verification from Authorization header
  - Scope-based access control
  - Principal extraction and context injection
  - Support for custom claims
- **API Key Auth** (`pkg/middleware`): Simple API key-based authentication middleware
- **Response Helper** (`pkg/helpers`): Helper functions for creating standardized API responses:
  - `Success()`: Success response with data and message
  - `Error()`: Error response with status code
  - `ValidationError()`: Validation error response
  - `BadRequest()`, `Unauthorized()`, `NotFound()`, `InternalError()`: Common HTTP error responses
- **Request Helper** (`pkg/helpers`): Utility functions for extracting information from Echo context:
  - `GetTraceId()`: Extract trace ID from request context
- **JWT Helper** (`pkg/helpers`): Helper functions for JWT operations in Echo context:
  - `VerifyToken()`: Verify and extract JWT token from Echo context

### Common

- **Base Response** (`pkg/common`): Standardized API response structure with success/error handling
- **I18n** (`pkg/common`): Internationalization support with locale management and message loading
- **Context Keys** (`pkg/common`): Context key definitions for request context management

### Services

- **JWT Service** (`pkg/service`): JWT token generation, validation, and management with claims handling

### Models

- **Auth Models** (`pkg/models`): Authentication and authorization data models (OAuthUser, JWTClaims)
- **RabbitMQ Models** (`pkg/models`): RabbitMQ configuration models (PublishOptions, ConsumeOptions, QueueOptions, DLXOptions, DLQOptions)

### Utilities

- **Errors** (`pkg/utils`): Custom error types and error handling utilities
- **Utils** (`pkg/utils`): JSON utilities and helper functions

### Version

- **Version** (`pkg/version`): Application version management and retrieval

## Development

### Building

```bash
make build
```

### Running Tests

```bash
make test
```

### Code Formatting

```bash
make fmt
```

### Running Linter

```bash
make vet
```

### All Checks

```bash
make check  # Runs fmt, vet, and test
```

See [Makefile](Makefile) for all available commands.

## Dependencies

### Core Dependencies

- [Echo](https://github.com/labstack/echo) - High performance HTTP web framework (Core framework for API development)
- [Redis Go Client](https://github.com/redis/go-redis) - Redis client for Go
- [RabbitMQ AMQP Client](https://github.com/rabbitmq/amqp091-go) - Official RabbitMQ client for Go
- [GORM](https://gorm.io/) - The fantastic ORM library for Go
- [GORM Drivers](https://gorm.io/docs/connecting_to_the_database.html) - Database drivers for MySQL, PostgreSQL, SQLite
- [OpenTelemetry](https://opentelemetry.io/) - Observability framework with Jaeger export support
- [Jaeger](https://www.jaegertracing.io/) - Distributed tracing backend (via OpenTelemetry exporter)
- [Logrus](https://github.com/sirupsen/logrus) - Structured logger
- [MinIO Go Client](https://github.com/minio/minio-go) - S3-compatible object storage
- [Cloudinary Go SDK](https://github.com/cloudinary/cloudinary-go) - Media management
- [JWT Go](https://github.com/golang-jwt/jwt) - JSON Web Token implementation

## Contributing

**Use issues for everything**

- For a small change, just send a PR
- For bigger changes open an issue for discussion before sending a PR
- PR should have:
  - Test case
  - Documentation
  - Example (if it makes sense)
- You can also contribute by:
  - Reporting issues
  - Suggesting new features or enhancements
  - Improve/fix documentation

## Usage in Other Projects

To use this core module in your microservices projects:

```bash
# In your project's go.mod
go get github.com/thanhthanh221/msa-core

# Or for local development
go mod edit -replace github.com/thanhthanh221/msa-core=./path/to/msa-core
```

Then import the packages you need:

```go
import (
    "github.com/thanhthanh221/msa-core/pkg/infrastructure/redis"
    "github.com/thanhthanh221/msa-core/pkg/infrastructure/rabbitmq"
    "github.com/thanhthanh221/msa-core/pkg/infrastructure/repositories"
    "github.com/thanhthanh221/msa-core/pkg/middleware"
    "github.com/thanhthanh221/msa-core/pkg/common"
    "github.com/thanhthanh221/msa-core/pkg/models"
)
```

## Examples

See the [examples](examples/) directory for more usage examples.

## License

[MIT](LICENSE)

---

Built with ❤️ for the MSA community

