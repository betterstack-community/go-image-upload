# Use Go 1.23 bookworm as base image
FROM golang:1.23-bookworm AS base

# Development stage
# =============================================================================
# Create a development stage based on the "base" image
FROM base AS development

# Change the working directory to /app
WORKDIR /app

RUN git config --global --add safe.directory /app

RUN apt-get update && apt-get install libaom-dev -y --no-install-recommends

# Install the air CLI for auto-reloading
RUN go install github.com/air-verse/air@latest

# Copy the go.mod and go.sum files to the /app directory
COPY go.mod go.sum ./

# Install dependencies
RUN go mod download

# Start air for live reloading
CMD ["air"]

# Builder stage
# =============================================================================
# Create a builder stage based on the "base" image
FROM base AS builder

# Move to working directory /build
WORKDIR /build

RUN apt-get update && apt-get install libaom-dev -y --no-install-recommends

# Copy the go.mod and go.sum files to the /build directory
COPY go.mod go.sum ./

# Install dependencies
RUN go mod download

# Copy the entire source code into the container
COPY . .

# Build the application
RUN go build -o go-social-login

# Production stage
# =============================================================================
# Create a production stage to run the application binary
FROM alpine:3.20 AS production

# Move to working directory /prod
WORKDIR /prod

# Copy binary from builder stage
COPY --from=builder /build/go-social-login ./

# Document the port that may need to be published
EXPOSE 8000

# Start the application
CMD ["/prod/go-social-login"]
