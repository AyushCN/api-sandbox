#!/bin/bash
set -e

echo "Running backend tests..."
cd backend
go test ./...
go vet ./...

echo "All tests passed successfully!"
