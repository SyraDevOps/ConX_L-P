@echo off
REM For the server
echo Starting server with custom API endpoint...
cd server
go run . -api "ws://your-vps-address:8000/ws/server/main-server"

REM For the client (in another terminal)
REM cd clients
REM go run . -relay "ws://your-vps-address:8000/ws/client/custom-client-id"