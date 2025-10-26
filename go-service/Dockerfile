# Build stage
FROM golang:1.25.3-alpine AS build

# Install build dependencies for CGO
RUN apk add --no-cache gcc musl-dev

WORKDIR /app

# Copy dependency files first for better layer caching
COPY go.mod go.sum ./

# Cache mounts speed up the installation of existing dependencies
RUN --mount=type=cache,target=/go/pkg/mod \
  --mount=type=cache,target=/root/.cache/go-build \
  go mod download

# Add non-root user (Alpine uses adduser instead of useradd)
RUN adduser -D -u 1001 altuser

COPY . .

# Compile the application as a statically-linked binary
RUN CGO_ENABLED=0 go build \
  -ldflags="-s -w" \
  -o echo-app

# Production stage using minimal Alpine image
FROM alpine:3.22.2

WORKDIR /

# Install Git and Terraform
RUN apk add --no-cache git curl unzip && \
  TERRAFORM_VERSION=1.9.8 && \
  curl -fsSLo terraform.zip https://releases.hashicorp.com/terraform/${TERRAFORM_VERSION}/terraform_${TERRAFORM_VERSION}_linux_amd64.zip && \
  unzip terraform.zip -d /usr/local/bin && \
  rm terraform.zip && \
  terraform version

COPY --from=build /app/echo-app echo-app

# Use non-root user
COPY --from=build /etc/passwd /etc/passwd
USER altuser

EXPOSE 8080
CMD ["/echo-app"]
