#!/bin/bash

# For the server
echo "Starting server with custom API endpoint..."
cd server
go run . -api "ws://your-vps-address:8000/ws/server/main-server"

# For the client (in another terminal)
# cd clients
# go run . -relay "ws://your-vps-address:8000/ws/client/custom-client-id"