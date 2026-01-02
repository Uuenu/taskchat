# Real-Time Chat Backend

A Go-based real-time chat backend with JWT authentication, MongoDB storage, and Tinode integration for real-time messaging.

## Features

- **User Authentication**: Sign up and login with JWT tokens
- **Real-Time Messaging**: Send and receive messages via Tinode integration
- **Message Persistence**: All messages stored in MongoDB
- **RESTful API**: Clean API endpoints for all operations
- **Graceful Degradation**: Service works without Tinode (messages still stored in MongoDB)

## Tech Stack

- **Go 1.21** with Gin framework
- **MongoDB 7.0** for data persistence
- **Tinode** for real-time messaging
- **JWT** for authentication
- **Docker & Docker Compose** for containerization

## Quick Start

### Prerequisites

- Docker and Docker Compose installed
- Git

### Run with Docker

```bash
# Clone the repository
git clone https://github.com/Uuenu/taskchat.git
cd taskchat

# Copy environment file
cp .env.example .env

# Start all services
docker-compose up -d

# View logs
docker-compose logs -f api
```

The API will be available at `http://localhost:8080`

### Local Development

```bash
# Start MongoDB and Tinode with Docker
docker-compose up -d mongodb tinode

# Set environment variables for local connection
export MONGODB_URI=mongodb://localhost:27017
export TINODE_HOST=localhost
export TINODE_PORT=6060

# Run the server
go run cmd/server/main.go
```

## API Endpoints

### Authentication

#### Sign Up
```bash
POST /signup
Content-Type: application/json

{
  "email": "user@example.com",
  "password": "password123"
}
```

Response (201):
```json
{
  "message": "User registered successfully"
}
```

#### Login
```bash
POST /login
Content-Type: application/json

{
  "email": "user@example.com",
  "password": "password123"
}
```

Response (200):
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
}
```

### Messaging (Requires Authentication)

Include the JWT token in the Authorization header:
```
Authorization: Bearer <your-jwt-token>
```

#### Send Message
```bash
POST /message
Authorization: Bearer <token>
Content-Type: application/json

{
  "content": "Hello, world!"
}
```

Response (200):
```json
{
  "message": "Message sent successfully"
}
```

#### Get Messages
```bash
GET /messages
Authorization: Bearer <token>
```

Response (200):
```json
[
  {
    "author": "user@example.com",
    "content": "Hello!",
    "timestamp": "2024-08-08T12:00:00Z"
  }
]
```

### Health Check
```bash
GET /health
```

Response (200):
```json
{
  "status": "healthy",
  "mongodb": "connected",
  "tinode": "connected"
}
```

## Testing with cURL

```bash
# 1. Register a new user
curl -X POST http://localhost:8080/signup \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"password123"}'

# 2. Login and save token
TOKEN=$(curl -s -X POST http://localhost:8080/login \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"password123"}' | jq -r '.token')

# 3. Send a message
curl -X POST http://localhost:8080/message \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"content":"Hello from Cody!"}'

# 4. Get messages
curl http://localhost:8080/messages \
  -H "Authorization: Bearer $TOKEN"

# 5. Check health
curl http://localhost:8080/health
```

## Project Structure

```
taskchat/
├── cmd/server/
│   └── main.go              # Application entry point
├── internal/
│   ├── config/
│   │   └── config.go        # Configuration management
│   ├── handlers/
│   │   ├── handlers.go      # HTTP request handlers
│   │   └── dto.go           # Request/Response DTOs
│   ├── middleware/
│   │   └── auth.go          # JWT authentication middleware
│   ├── models/
│   │   └── models.go        # Domain models
│   ├── repository/
│   │   ├── interfaces.go    # Repository interfaces
│   │   └── mongo/           # MongoDB implementations
│   ├── service/
│   │   ├── auth.go          # Authentication service (includes JWT)
│   │   ├── chat.go          # Chat service
│   │   └── tinode.go        # Tinode integration
│   └── mocks/               # Test mocks
├── docker-compose.yml       # Docker services
├── Dockerfile               # Multi-stage build
├── Makefile                 # Build automation
└── README.md
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `SERVER_PORT` | 8080 | HTTP server port |
| `MONGODB_URI` | mongodb://mongodb:27017 | MongoDB connection string |
| `MONGODB_DATABASE` | chat_db | Database name |
| `TINODE_HOST` | tinode | Tinode server host |
| `TINODE_PORT` | 6060 | Tinode server port |
| `TINODE_API_KEY` | AQEAAAABAAD_rAp4DJh05a1HAwFT3A6K | Tinode API key |
| `TINODE_SERVICE_LOGIN` |  | Service login for backend Tinode session (optional) |
| `TINODE_SERVICE_PASSWORD` |  | Service password for backend Tinode session (optional) |
| `JWT_SECRET` | change-in-prod | Secret key for JWT signing |
| `GIN_MODE` | debug | Gin framework mode |

## Makefile Commands

```bash
make build        # Build the application binary
make run          # Run locally
make test         # Run tests
make up           # Start all services with Docker
make down         # Stop Docker services
make docker-logs  # View logs
make api-test     # Quick API smoke test
```

## Security Notes

- Change `JWT_SECRET` in production
- Use HTTPS in production
- Passwords are hashed using bcrypt
- Consider rate limiting for production
