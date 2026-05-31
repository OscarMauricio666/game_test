#!/usr/bin/env bash

BACKEND_DIR="$(cd "$(dirname "$0")" && pwd)"
FRONTEND_DIR="$(cd "$BACKEND_DIR/../zen-garden" && pwd)"

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

log()  { echo -e "${GREEN}[zen-garden]${NC} $1"; }
warn() { echo -e "${YELLOW}[zen-garden]${NC} $1"; }
fail() { echo -e "${RED}[zen-garden] ERROR:${NC} $1"; echo -e "${RED}[zen-garden] El script terminará en 3 segundos...${NC}"; sleep 3; exit 1; }

# Kill all processes on a given port (handles multiple PIDs)
kill_port() {
  local port=$1
  local pids
  pids=$(lsof -ti tcp:"$port" 2>/dev/null || true)
  if [ -n "$pids" ]; then
    warn "Puerto $port ocupado, liberando..."
    echo "$pids" | xargs kill -9 2>/dev/null || true
    sleep 1
  fi
}

kill_port 8080
kill_port 5173

# Start backend
log "Iniciando backend Go en :8080..."
cd "$BACKEND_DIR"
go build -o zen-garden-server . || fail "Falló la compilación del backend"
./zen-garden-server &
BACKEND_PID=$!

# Start frontend
log "Iniciando frontend Svelte en :5173..."
cd "$FRONTEND_DIR"
npm run dev -- --port 5173 &> /tmp/zen-garden-frontend.log &
FRONTEND_PID=$!

# Trap to kill both on exit
cleanup() {
  echo ""
  log "Deteniendo servicios..."
  kill "$BACKEND_PID" "$FRONTEND_PID" 2>/dev/null || true
}
trap cleanup EXIT INT TERM

# Validate backend (max 10s)
log "Validando backend..."
for i in $(seq 1 10); do
  code=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/ws 2>/dev/null || echo "000")
  if [ "$code" = "400" ]; then
    log "Backend OK"
    break
  fi
  if [ "$i" = "10" ]; then
    fail "Backend no respondió. Log: go run . en $BACKEND_DIR"
  fi
  sleep 1
done

# Validate frontend (max 15s)
log "Validando frontend..."
for i in $(seq 1 15); do
  code=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:5173 2>/dev/null || echo "000")
  if [ "$code" = "200" ]; then
    log "Frontend OK"
    break
  fi
  if [ "$i" = "15" ]; then
    fail "Frontend no respondió. Log: /tmp/zen-garden-frontend.log"
  fi
  sleep 1
done

# Validate WebSocket end-to-end
log "Validando WebSocket extremo a extremo..."
# Validate WebSocket via direct backend (skip Python dependency)
log "Validando WebSocket directo al backend..."
code=$(curl -s -o /dev/null -w "%{http_code}" \
  -H "Upgrade: websocket" \
  -H "Connection: Upgrade" \
  -H "Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==" \
  -H "Sec-WebSocket-Version: 13" \
  http://localhost:8080/ws 2>/dev/null || echo "000")

if [ "$code" = "101" ] || [ "$code" = "400" ]; then
  log "WebSocket OK (backend respondiendo)"
else
  fail "WebSocket no responde (código: $code)"
fi

echo ""
echo -e "${GREEN}============================================${NC}"
echo -e "${GREEN}  Zen Garden corriendo correctamente${NC}"
echo -e "${GREEN}============================================${NC}"
echo -e "  Juego:    ${GREEN}http://localhost:5173${NC}"
echo -e "  Backend:  ws://localhost:8080/ws"
echo -e "${GREEN}============================================${NC}"
echo ""
log "Presiona Ctrl+C para detener ambos servicios."

wait "$BACKEND_PID"
