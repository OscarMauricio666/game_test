#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")" && pwd)"

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

log()  { echo -e "${GREEN}[zen-garden]${NC} $1"; }
warn() { echo -e "${YELLOW}[zen-garden] WARN:${NC} $1"; }
fail() {
  echo -e "${RED}[zen-garden] ERROR:${NC} $1" >&2
  echo "" >&2
  echo -e "${RED}--- Logs del stack ---${NC}" >&2
  docker compose -f "$ROOT_DIR/docker-compose.yml" logs --tail=30 2>/dev/null || true
  echo -e "${RED}--- Fin de logs ---${NC}" >&2
  echo "" >&2
  echo -e "${YELLOW}Para más detalles:${NC}  docker compose logs -f" >&2
  exit 1
}

# ── Prerequisitos ──────────────────────────────────────────────────────────────

if ! command -v docker &>/dev/null; then
  fail "Docker no está instalado. Descárgalo desde https://docs.docker.com/get-docker/"
fi

if ! docker compose version &>/dev/null 2>&1; then
  fail "Docker Compose no está disponible. Actualiza Docker Desktop."
fi

if ! docker info &>/dev/null 2>&1; then
  fail "El daemon de Docker no está corriendo. Inicia Docker Desktop e intenta de nuevo."
fi

# ── Build y despliegue ─────────────────────────────────────────────────────────

log "Construyendo imágenes y levantando el stack..."
if ! docker compose -f "$ROOT_DIR/docker-compose.yml" up --build -d 2>&1; then
  fail "Falló el build o el inicio de los contenedores."
fi

# ── Validar frontend ───────────────────────────────────────────────────────────

log "Esperando que el frontend esté listo..."
FRONTEND_OK=false
for i in $(seq 1 20); do
  code=$(curl -s -o /dev/null -w "%{http_code}" http://localhost 2>/dev/null || echo "000")
  if [ "$code" = "200" ]; then
    FRONTEND_OK=true
    break
  fi
  sleep 2
done

if [ "$FRONTEND_OK" = false ]; then
  fail "El frontend no respondió en http://localhost después de 40 segundos."
fi
log "Frontend OK → http://localhost"

# ── Validar WebSocket (a través de nginx) ──────────────────────────────────────

log "Validando WebSocket a través de nginx..."
WS_OK=false
for i in $(seq 1 5); do
  code=$(curl -s -o /dev/null -w "%{http_code}" \
    -H "Upgrade: websocket" \
    -H "Connection: Upgrade" \
    -H "Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==" \
    -H "Sec-WebSocket-Version: 13" \
    http://localhost/ws 2>/dev/null || echo "000")
  if [ "$code" = "101" ] || [ "$code" = "400" ]; then
    WS_OK=true
    break
  fi
  sleep 1
done

if [ "$WS_OK" = false ]; then
  warn "El proxy WebSocket no respondió como se esperaba. El juego puede no funcionar."
  warn "Revisa los logs del backend: docker compose logs backend"
else
  log "WebSocket OK → ws://localhost/ws"
fi

# ── Estado de los contenedores ─────────────────────────────────────────────────

echo ""
echo -e "${CYAN}Estado de los contenedores:${NC}"
docker compose -f "$ROOT_DIR/docker-compose.yml" ps
echo ""

# ── Resumen final ──────────────────────────────────────────────────────────────

echo -e "${GREEN}============================================${NC}"
echo -e "${GREEN}  Zen Garden desplegado correctamente${NC}"
echo -e "${GREEN}============================================${NC}"
echo -e "  Juego:      ${GREEN}http://localhost${NC}"
echo -e "  WebSocket:  ws://localhost/ws"
echo -e ""
echo -e "  Logs:       docker compose logs -f"
echo -e "  Detener:    docker compose down"
echo -e "${GREEN}============================================${NC}"
