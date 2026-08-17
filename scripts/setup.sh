#!/bin/bash
set -e

echo "🚀 API Sandbox Setup"

# Check prerequisites
if ! command -v docker &> /dev/null; then
    echo "❌ Docker not found. Install Docker Desktop first."
    exit 1
fi

if ! command -v docker-compose &> /dev/null; then
    echo "❌ Docker Compose not found."
    exit 1
fi

# Create workspaces directory with proper permissions
WORKSPACES_DIR="/var/lib/api-sandbox/workspaces"
if [ ! -d "$WORKSPACES_DIR" ]; then
    echo "📁 Creating workspaces directory..."
    sudo mkdir -p "$WORKSPACES_DIR"
    sudo chmod 777 "$WORKSPACES_DIR"
fi

# Copy .env.example if .env doesn't exist
if [ ! -f .env ]; then
    echo "📝 Creating .env from .env.example"
    cp .env.example .env
    echo "⚠️  Please edit .env and set:"
    echo "   - GITHUB_CLIENT_ID"
    echo "   - GITHUB_CLIENT_SECRET"
    echo "   - TOKEN_ENCRYPTION_KEY (generate: openssl rand -hex 16)"
    echo "   - JWT_SECRET (generate: openssl rand -hex 32)"
    exit 1
fi

# Build and start
echo "🔨 Building containers..."
docker-compose up -d --build

# Wait for backend health
echo "⏳ Waiting for backend to be healthy..."
timeout 60 bash -c 'until curl -s http://localhost:8080/health; do sleep 2; done' || { echo "❌ Backend failed to start"; exit 1; }

echo "✅ API Sandbox is ready!"
echo "📍 Open http://localhost in your browser"
