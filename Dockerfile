# Official Go Alpine Base Image for building the application
FROM golang:1.24-alpine AS builder

# Set the working directory inside the container
WORKDIR /app

# Copy the Go modules files and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the entire source code into the container
COPY . .

# Build the Go binary
RUN go build -o api cmd/server/main.go

# Final Image Creation Stage using a lightweight Alpine image
FROM alpine:3.21

# Set the working directory
WORKDIR /root/

# Install necessary dependencies (bash for scripts, curl for consul health checks)
RUN apk add --no-cache libc6-compat bash curl

# Copy the built Go binary from the builder image
COPY --from=builder /app/api .

# Copy the .env file to the container
COPY ./.env /root/.env

# Copy credentials
COPY ./credentials /root/credentials

# Copy the original wait-for-it.sh script (for MongoDB)
COPY ./scripts/wait-for-it.sh /wait-for-it.sh
RUN chmod +x /wait-for-it.sh

# Copy the new wait-for-consul.sh script
COPY ./scripts/wait-for-consul.sh /wait-for-consul.sh
RUN chmod +x /wait-for-consul.sh

# Copy the startup chain script
COPY ./scripts/start-services.sh /start-services.sh
RUN chmod +x /start-services.sh

# Expose the necessary port
EXPOSE 8007

# Use the startup chain script as the entrypoint
# This will wait for MongoDB first, then Consul, then start the app
CMD ["/start-services.sh", "./api"]