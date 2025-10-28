# Prism

**Visualize your infrastructure and interact with it using natural language**

Prism is a CalHacks 12.0 project that enables developers and DevOps teams to visualize their infrastructure and interact with it through natural language interfaces. Built with modern web technologies and a microservices architecture, Prism provides an intuitive way to manage and understand complex infrastructure setups.

## Technology Stack

### Frontend
- **Next.js 15** - React framework with App Router and Turbopack
- **React 19** - Modern UI library
- **TypeScript** - Type-safe development
- **Tailwind CSS** - Utility-first styling
- **tRPC** - End-to-end type-safe APIs
- **NextAuth.js** - Authentication with Discord provider
- **@paper-design/shaders-react** - Visual effects and animations

### Backend
- **Go** - High-performance backend service
- **Echo** - Web framework for Go
- **GORM** - ORM for database operations

### Database & Storage
- **PostgreSQL** - Primary database
- **Drizzle ORM** - TypeScript-first ORM for Next.js
- **MinIO** - S3-compatible object storage

### Infrastructure & DevOps
- **Docker & Docker Compose** - Containerization and orchestration
- **Infisical** - Secrets management
- **Redis** - Caching (used by Infisical)

### Additional Services
- **LLM Integration** - Natural language processing capabilities
- **Orchestrator** - Service coordination and workflow management

## Architecture

Prism follows a microservices architecture with the following components:

```
┌─────────────────┐     ┌──────────────────┐
│   Next.js App   │────▶│   Go Service     │
│   (Port 3000)   │     │   (Port 1323)    │
└────────┬────────┘     └────────┬─────────┘
         │                       │
         │                       │
    ┌────▼────────────────────┬──▼─────┐
    │                         │        │
┌───▼────────┐    ┌──────────▼──┐  ┌──▼─────────┐
│ PostgreSQL │    │   MinIO     │  │ Infisical  │
│ (Port 5432)│    │(Ports 9000, │  │(Port 8080) │
└────────────┘    │     9001)   │  └────────────┘
                  └─────────────┘
```

- **Next.js Frontend**: Handles user interface, authentication, and client-side logic
- **Go Backend**: Provides REST APIs for infrastructure management, LLM operations, and orchestration
- **PostgreSQL**: Stores application data and user information
- **MinIO**: Object storage for artifacts and files
- **Infisical**: Centralized secrets management for secure credential storage

## Prerequisites

- Docker and Docker Compose
- Node.js 22+ and npm
- Go 1.25+ (for local development)

## Getting Started

### 1. Environment Setup

Copy the example environment file:
```bash
cp .env.example .env
```

Fill in the required environment variables in `.env`. You'll need to complete the Infisical setup below to get `INFISICAL_CLIENT_ID` and `INFISICAL_CLIENT_SECRET`.

### 2. Infisical Setup

Before running the full application, set up Infisical for secrets management:

1. Start the Infisical service:
   ```bash
   docker compose up infisical-secrets-manager infisical-db infisical-redis
   ```

2. Access Infisical at `http://localhost:8080` and complete the initial setup

3. Configure a machine identity (required for the Go SDK):
   - Go to **Org** → **Access Control** → **Identities** → Create Identity with **Member Role**
   - Go to **Secrets Manager** → **Access Management** → **Machine Identities** → Add Identity → Select with **Developer Role**
   - Go to **Org** → **Access Control** → **Identities** → Click your Identity → **Universal Auth**
   - Copy the **Client ID** and create a **Client Secret**, then copy it
   - Add these values to your `.env` file as `INFISICAL_CLIENT_ID` and `INFISICAL_CLIENT_SECRET`

### 3. Running the Application

Once Infisical is configured, start all services:

```bash
docker compose up
```

Or use the npm script:
```bash
npm run dev
```

This will start:
- **PostgreSQL database** (port 5432)
- **MinIO object storage** (ports 9000, 9001)
- **Infisical secrets manager** (port 8080)
- **Go service** (port 1323)
- **Next.js application** (port 3000)

Access the application at `http://localhost:3000`

## Development

### Frontend Development

```bash
# Install dependencies
npm install

# Run Next.js dev server (standalone)
npm run start

# Run linting
npm run lint

# Run type checking
npm run typecheck

# Format code
npm run format:write
```

### Database Management

```bash
# Generate database migrations
npm run db:generate

# Push schema changes
npm run db:push

# Open Drizzle Studio
npm run db:studio
```

### Backend Development

```bash
# Navigate to Go service
cd go-service

# Install dependencies
go mod download

# Run the service
go run .
```

## 📁 Project Structure

```
prism/
├── src/
│   ├── app/              # Next.js App Router pages
│   │   ├── api/          # API routes
│   │   ├── dashboard/    # Dashboard interface
│   │   └── page.tsx      # Landing page
│   ├── components/       # React components
│   ├── lib/             # Utility functions and constants
│   ├── server/          # Server-side code
│   │   ├── api/         # tRPC routers
│   │   ├── auth/        # Authentication configuration
│   │   └── db/          # Database schema and client
│   ├── styles/          # Global styles
│   └── trpc/            # tRPC client configuration
├── go-service/
│   ├── internal/        # Go service modules
│   │   ├── infisical/   # Infisical integration
│   │   ├── llm/         # LLM service
│   │   ├── minio/       # MinIO client
│   │   └── orchestrator/ # Orchestration logic
│   ├── main.go          # Go service entry point
│   └── routes.go        # API route definitions
├── compose.yaml         # Docker Compose configuration
├── Dockerfile           # Next.js Dockerfile
└── package.json         # Node.js dependencies
```

## Useful Links

- [Next.js Documentation](https://nextjs.org/docs)
- [T3 Stack](https://create.t3.gg/)
- [Echo Framework](https://echo.labstack.com/)
- [Drizzle ORM](https://orm.drizzle.team)
- [Infisical](https://infisical.com/docs)
- [MinIO](https://min.io/docs/minio/linux/index.html)

## License

This project was created for CalHacks 12.0.

## Contributing

This is a hackathon project. Feel free to fork and experiment!
