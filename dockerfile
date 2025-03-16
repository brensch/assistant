# Stage 1: Build the Go application
FROM golang:1.24-bullseye AS builder

WORKDIR /app

# Install build dependencies for DuckDB
RUN apt-get update && apt-get install -y \
    gcc \
    g++ \
    libc-dev \
    build-essential \
    && rm -rf /var/lib/apt/lists/*

# Copy go.mod and go.sum files first to leverage Docker cache
COPY go.mod go.sum* ./
RUN go mod download

# Copy the source code
COPY . .

# Build the application - using partial static linking
RUN CGO_ENABLED=1 GOOS=linux go build -o main .

# Stage 2: Create the minimal runtime image
FROM debian:bullseye-slim

WORKDIR /app

# Install runtime dependencies for DuckDB
RUN apt-get update && apt-get install -y \
    ca-certificates \
    libstdc++6 \
    && rm -rf /var/lib/apt/lists/*

# Create directories for configuration and data
RUN mkdir -p /app/data

# Define volumes for configuration and data
VOLUME ["/app/data"]

# Copy the binary from the builder stage
COPY --from=builder /app/main .

# Set the data directory environment variable
ENV DATA_DIR=/app/data

# Run the application
CMD ["./main"]