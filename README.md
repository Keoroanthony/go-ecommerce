Project Structure
.
├── configs/             # Application configuration setup
├── docker-compose.yml   # Docker Compose for local development (Go app + PostgreSQL)
├── Dockerfile           # Dockerfile for building the Go application image
├── go.mod               # Go module definition
├── go.sum               # Go module checksums
├── internal/            # Internal packages containing core application logic
│   ├── auth/            # Authentication logic (OIDC)
│   ├── db/              # Database connection and setup (PostgreSQL)
│   ├── handlers/        # HTTP request handlers
│   │   └── tests/       # Dedicated directory for handler integration tests
│   ├── models/          # Database models (structs mapping to DB tables)
│   ├── notifier/        # Notification service integrations (SMS, Email)
│   └── utils/           # Utility functions (e.g., category hierarchy traversal)
├── k8s/                 # Kubernetes manifests for deployment
├── main.go              # Main application entry point
└── README.md            # Project README file


Getting Started
Follow these instructions to set up and run the application locally or deploy it.

Prerequisites
Go: Go 1.23.4 or newer.
Docker: Docker Desktop or Docker Engine.
Docker Compose: Usually comes with Docker Desktop.
kubectl: Kubernetes command-line tool.
minikube / kind: For local Kubernetes deployment (choose one).
minikube installation guide
kind installation guide
1. Local Development (without Docker)
If you want to run the Go application directly on your machine, you'll need a local PostgreSQL database.

Clone the repository:
Bash

git clone https://github.com/Keoroanthony/go-ecommerce.git
cd go-ecommerce
Install Go Modules:
Bash

go mod download
Set Environment Variables: Your application relies on several environment variables for database connection, OIDC, and notifier services. Create a .env file (not committed to Git!) or set these in your shell.
Bash

# Example .env file (request actual values)
DB_HOST=localhost
DB_PORT=5432
DB_USER=your_db_user
DB_PASSWORD=your_db_password
DB_NAME=your_db_name

OIDC_ISSUER_URL=https://accounts.google.com # Or your actual OIDC provider
OIDC_CLIENT_ID=your-oidc-client-id
OIDC_CLIENT_SECRET=your-oidc-client-secret
OIDC_REDIRECT_URL=http://localhost:8080/auth/callback

SESSION_SECRET=a-long-and-random-string-for-session-secret

AT_SENDER_ID=your-at-sender-id
AT_USERNAME=your-at-username
AT_API_KEY=your-at-api-key

AWS_ACCESS_KEY_ID=your-aws-access-key-id
AWS_SECRET_ACCESS_KEY=your-aws-secret-access-key
AWS_REGION=us-east-1
If using a .env file, you might need a tool like direnv or explicitly source it.
Start PostgreSQL: Ensure a PostgreSQL instance is running locally and is accessible using the credentials above.
Run the application:
Bash

go run .
The application should start on http://localhost:8080.