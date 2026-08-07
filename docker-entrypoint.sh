#!/bin/sh
set -e

# Start Go Backend in the background on port 8080
echo "Starting CHANAKYA Go Backend on port 8080..."
/app/api &

# Wait briefly for backend to initialize SQLite database
sleep 2

# Start Next.js Frontend server in the foreground on $PORT (Cloud Run passes PORT env var)
echo "Starting CHANAKYA Next.js Frontend on port ${PORT:-3000}..."
exec node frontend/apps/web/server.js
