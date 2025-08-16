#!/usr/bin/env bash
# Startup script to wait for all dependencies before starting the application

set -e

# Colors for logging
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

# Configuration
MONGO_HOST=${MONGO_HOST:-chat_db}
MONGO_PORT=${MONGO_PORT:-27017}
CONSUL_HOST=${CONSUL_HOST:-consul}
CONSUL_PORT=${CONSUL_PORT:-8500}
TIMEOUT=${TIMEOUT:-60}

log_info "=== Chat Service Startup ==="
log_info "Dependencies to wait for:"
log_info "  - MongoDB: $MONGO_HOST:$MONGO_PORT"
log_info "  - Consul:  $CONSUL_HOST:$CONSUL_PORT"
log_info "  - Timeout: ${TIMEOUT}s each"
log_info ""

# Step 1: Wait for MongoDB
log_info "Step 1/2: Waiting for MongoDB..."
if /wait-for-it.sh $MONGO_HOST:$MONGO_PORT -t $TIMEOUT; then
    log_success "MongoDB is ready!"
else
    log_error "MongoDB failed to start within ${TIMEOUT}s"
    exit 1
fi

log_info ""

# Step 2: Wait for Consul
log_info "Step 2/2: Waiting for Consul..."
if /wait-for-consul.sh $CONSUL_HOST:$CONSUL_PORT -t $TIMEOUT; then
    log_success "Consul is ready!"
else
    log_error "Consul failed to start within ${TIMEOUT}s"
    exit 1
fi

log_info ""
log_success "All dependencies are ready! Starting application..."
log_info "Executing: $@"
log_info ""

# Execute the main application
exec "$@"