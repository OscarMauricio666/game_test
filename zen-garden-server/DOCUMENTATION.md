# Zen Garden Server - Documentación Backend

## Descripción General

Zen Garden Server es el backend de un juego multijugador en tiempo real estilo **Agar.io**, construido en **Go**. Gestiona conexiones WebSocket, la simulación del mundo del juego, detección de colisiones y broadcast del estado a todos los clientes conectados.

---

## Estructura del Proyecto

```
zen-garden-server/
├── main.go       → Punto de entrada, servidor HTTP y endpoint WebSocket
├── hub.go        → Gestión de clientes WebSocket (read/write pumps)
├── game.go       → Motor del juego: loop principal, luces, colisiones, estado
├── player.go     → Entidad jugador: movimiento, crecimiento, velocidad
├── go.mod        → Definición del módulo Go
└── go.sum        → Lock de dependencias
```

### Dependencias

| Dependencia | Versión | Uso |
|---|---|---|
| `github.com/gorilla/websocket` | v1.5.3 | Manejo de conexiones WebSocket |

---

## Arquitectura

```
                    ┌──────────────────┐
                    │   main.go        │
                    │  HTTP :8080      │
                    │  GET /ws         │
                    └────────┬─────────┘
                             │ Upgrade a WebSocket
                             ▼
                    ┌──────────────────┐
                    │    hub.go        │
                    │  Client{conn,    │
                    │   send, player}  │
                    │  readPump()      │
                    │  writePump()     │
                    └────────┬─────────┘
                             │ Canales (register, unregister, join)
                             ▼
              ┌──────────────────────────────┐
              │          game.go             │
              │   Game Loop (20 ticks/seg)   │
              │   ├── Spawn luces            │
              │   ├── Update jugadores       │
              │   ├── Detectar colisiones    │
              │   ├── Calcular ranking       │
              │   └── Broadcast estado       │
              └──────────────┬───────────────┘
                             │
                             ▼
                    ┌──────────────────┐
                    │   player.go      │
                    │   Movimiento     │
                    │   Crecimiento    │
                    │   Velocidad      │
                    └──────────────────┘
```

---

## Archivos en Detalle

### `main.go` - Punto de Entrada

Responsabilidades:
1. Crea una instancia de `Game`
2. Inicia el game loop en una goroutine separada (`go game.Run()`)
3. Registra el endpoint `/ws` para upgrades de WebSocket
4. Escucha en el puerto `:8080`

```go
func main() {
    game := NewGame()
    go game.Run()
    http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
        serveWs(game, w, r)
    })
    http.ListenAndServe(":8080", nil)
}
```

---

### `hub.go` - Gestión de Clientes

#### Struct `Client`

```go
type Client struct {
    conn   *websocket.Conn   // Conexión WebSocket
    send   chan []byte        // Canal de envío (buffer: 256)
    game   *Game              // Referencia al juego
    player *Player            // Jugador asociado (nil hasta join)
}
```

#### Constantes de Conexión

| Constante | Valor | Descripción |
|---|---|---|
| `writeWait` | 10s | Deadline para escribir mensajes |
| `pongWait` | 60s | Deadline para respuesta pong |
| `pingPeriod` | 54s | Frecuencia de envío de pings |

#### Funciones Clave

- **`readPump()`**: Goroutine que lee mensajes del cliente continuamente. Decodifica JSON y rutea según el tipo de mensaje (`join`, `input`, `target`).
- **`writePump()`**: Goroutine que envía mensajes al cliente desde el canal `send`. Maneja ping/pong para keep-alive.
- **`serveWs()`**: Upgradea la conexión HTTP a WebSocket, crea un `Client` y lo registra en el juego.

---

### `game.go` - Motor del Juego

#### Struct `Game`

```go
type Game struct {
    clients    map[*Client]bool      // Clientes conectados
    register   chan *Client           // Canal de registro
    unregister chan *Client           // Canal de desregistro
    join       chan *JoinRequest      // Canal de unirse al juego
    players    map[string]*Player     // Jugadores activos (por ID)
    lights     map[string]*Light      // Luces en el mapa
    nextID     int                    // Contador para IDs únicos
}
```

#### Constantes del Juego

| Constante | Valor | Descripción |
|---|---|---|
| `MapWidth` | 4000.0 | Ancho del mapa en unidades |
| `MapHeight` | 4000.0 | Alto del mapa en unidades |
| `TickRate` | 20 | Actualizaciones por segundo |
| `MaxLights` | 40 | Máximo de luces simultáneas |

#### Struct `Light`

```go
type Light struct {
    ID   string  `json:"id"`
    X    float64 `json:"x"`
    Y    float64 `json:"y"`
    Size float64 `json:"size"`   // 6 + random[0, 10)
}
```

#### Game Loop (`Run()`)

El game loop corre en una goroutine y ejecuta cada 50ms (20 ticks/segundo):

```
Cada 50ms:
├── Manejar eventos de canales:
│   ├── register   → Añadir cliente
│   ├── unregister → Remover cliente y jugador
│   └── join       → Crear jugador, enviar "welcome"
├── Timer de spawn (cada 2-5 segundos):
│   └── Crear luz si hay menos de 40
└── tick() → Actualización principal
    ├── Actualizar posición de todos los jugadores
    ├── Detectar colisiones jugador-luz
    ├── Construir mensaje de estado
    ├── Calcular ranking (top 10 por tamaño)
    └── Broadcast estado a todos los clientes
```

#### Detección de Colisiones

```
Para cada luz y cada jugador:
  distancia = sqrt((jugador.x - luz.x)² + (jugador.y - luz.y)²)
  Si distancia < jugador.size + luz.size:
    → Jugador crece: luz.size * 0.3
    → Luz se elimina del mapa
```

#### Spawn de Luces

- **Inicial**: 15 luces al crear el juego
- **Continuo**: Cada 2-5 segundos si hay menos de 40
- **Posición**: Aleatoria dentro del mapa (margen de 50 unidades del borde)
- **Tamaño**: `6 + random[0, 10)` unidades

---

### `player.go` - Entidad Jugador

#### Struct `Player`

```go
type Player struct {
    mu        sync.RWMutex
    ID        string
    Name      string       // Máximo 20 caracteres
    X, Y      float64      // Posición en el mapa
    Size      float64      // Tamaño inicial: 15.0
    Color     string       // Color hexadecimal de paleta
    dx, dy    float64      // Vector de dirección normalizado
    targetX, targetY float64  // Posición objetivo (click-to-move)
    hasTarget bool            // Si se está moviendo a un objetivo
}
```

#### Paleta de Colores

16 colores predefinidos asignados cíclicamente a los jugadores:

```
#ff6b6b  #ffd93d  #6bcb77  #4d96ff  #ff922b  #cc5de8
#20c997  #339af0  #f06595  #fcc419  #51cf66  #845ef7
#22b8cf  #ff6348  #a9e34b  #e599f7
```

#### Mecánica de Movimiento

**Dos modos de movimiento:**

1. **Dirección directa (WASD/Flechas)**: `SetDirection(dx, dy)` - Vector normalizado
2. **Click-to-move**: `SetTarget(x, y)` - Calcula dirección hacia el objetivo

**Cálculo de velocidad:**

```
velocidad = 200.0 / (1.0 + (size - 15) * 0.008)
```

> Los jugadores m��s grandes se mueven más lento.

**Actualización por tick:**

```
Si tiene objetivo:
  → Calcular dirección hacia el objetivo
  → Si está a menos de 3 unidades → limpiar objetivo
Mover: nueva_pos = pos + dirección * velocidad * dt
Clampar a los límites del mapa (0 a mapW/mapH)
```

#### Crecimiento

- Tamaño inicial: **15.0** unidades
- Al recolectar una luz: crece **30% del tamaño de la luz**
- A mayor tamaño, menor velocidad (relación inversa)

---

## Protocolo WebSocket

### Mensajes del Cliente → Servidor

#### 1. `join` - Unirse al juego
```json
{
    "type": "join",
    "name": "NombreJugador"
}
```

#### 2. `input` - Movimiento por teclado
```json
{
    "type": "input",
    "dx": -1.0,
    "dy": 0.5
}
```

#### 3. `target` - Click-to-move
```json
{
    "type": "target",
    "x": 2000.0,
    "y": 1500.0
}
```

### Mensajes del Servidor → Cliente

#### 1. `welcome` - Bienvenida tras unirse
```json
{
    "type": "welcome",
    "id": "p1",
    "mapW": 4000.0,
    "mapH": 4000.0
}
```

#### 2. `state` - Estado del juego (20 veces/segundo)
```json
{
    "type": "state",
    "players": [
        {
            "id": "p1",
            "name": "Alice",
            "x": 500.0,
            "y": 600.0,
            "size": 25.5,
            "color": "#ff6b6b"
        }
    ],
    "lights": [
        {
            "id": "l1",
            "x": 1000.0,
            "y": 1200.0,
            "size": 8.5
        }
    ],
    "top": [
        { "name": "Alice", "size": 25.5 },
        { "name": "Bob", "size": 20.3 }
    ]
}
```

---

## Concurrencia y Sincronización

| Mecanismo | Uso |
|---|---|
| `sync.RWMutex` en Player | Protege el estado del jugador (lecturas/escrituras concurrentes) |
| Game loop en goroutine única | Todo el estado del juego se gestiona en un solo hilo |
| Canales (register, unregister, join) | Comunicación sin bloqueo entre goroutines |
| Canal `send` (buffer 256) | Envío de mensajes a clientes sin bloquear el game loop |
| `readPump` / `writePump` por cliente | Goroutines dedicadas para I/O de WebSocket |

---

## Flujo de Datos

```
Navegador                       Servidor Go
   │                                │
   │──── WebSocket Connect ────────►│ serveWs() → Client{} → register
   │                                │
   │──── { type: "join" } ────────►│ join channel → NewPlayer() → welcome
   │                                │
   │◄─── { type: "welcome" } ──────│
   │                                │
   │──── { type: "input" } ───────►│ SetDirection()
   │──── { type: "target" } ──────►│ SetTarget()
   │                                │
   │         Cada 50ms:             │
   │                                │ tick() → Update → Colisiones → Estado
   │◄─── { type: "state" } ────────│ Broadcast a todos
   │                                │
```

---

## Cómo Ejecutar

### Docker (recomendado para pruebas y producción)

Desde la raíz del proyecto (`game_test/`):

```bash
./deploy.sh
```

El script:
1. Verifica que Docker esté instalado y corriendo
2. Construye las imágenes de backend (Go multi-stage) y frontend (Svelte → nginx)
3. Levanta ambos contenedores en segundo plano
4. Espera a que el frontend responda en `http://localhost`
5. Valida el proxy WebSocket a través de nginx
6. Muestra los últimos 30 líneas de logs de ambos servicios si algo falla

Salida esperada:

```
[zen-garden] Construyendo imágenes y levantando el stack...
[zen-garden] Esperando que el frontend esté listo...
[zen-garden] Frontend OK → http://localhost
[zen-garden] Validando WebSocket a través de nginx...
[zen-garden] WebSocket OK → ws://localhost/ws

============================================
  Zen Garden desplegado correctamente
============================================
  Juego:      http://localhost
  WebSocket:  ws://localhost/ws

  Logs:       docker compose logs -f
  Detener:    docker compose down
============================================
```

Comandos útiles mientras el stack corre:

```bash
docker compose logs -f          # logs en tiempo real
docker compose logs backend     # solo backend Go
docker compose ps               # estado de contenedores
docker compose down             # detener todo
```

### Desarrollo local (sin Docker)

```bash
# Backend (desde zen-garden-server/)
go run .
# → Escucha en :8080

# Frontend (desde zen-garden/, en otra terminal)
npm install && npm run dev
# → App en http://localhost:5173
```

O ambos servicios con un solo comando:

```bash
cd zen-garden-server && ./start.sh
```

El servidor escucha en `http://localhost:8080` con el endpoint WebSocket en `/ws`.
