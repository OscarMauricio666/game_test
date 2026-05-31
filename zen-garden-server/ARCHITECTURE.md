# Luces Errantes - Arquitectura General del Sistema

## Qué es este juego

**Luces Errantes** es un juego multijugador en tiempo real estilo Agar.io donde los jugadores deambulan por un mapa oscuro recolectando luces para crecer. A mayor tamaño, menor velocidad. El top 10 de jugadores se muestra en un ranking en tiempo real.

---

## Vista General del Sistema

```
┌──────────────────────────────┐         WebSocket          ┌──────────────────────────────┐
│        FRONTEND              │◄──────────────────────────►│         BACKEND              │
│  zen-garden/                 │    ws://localhost:8080/ws   │  zen-garden-server/          │
│                              │                             │                              │
│  Svelte 5 + Canvas 2D       │  ───── join ────────►       │  Go + Gorilla WebSocket      │
│  Renderizado procedural      │  ───── input ──────►       │  Game loop 20 ticks/s        │
│  Partículas y efectos        │  ───── target ─────►       │  Colisiones y física          │
│  UI: ranking, minimapa       │  ◄──── welcome ────        │  Spawn de luces              │
│                              │  ◄──── state ──────        │  Ranking top 10              │
│  Puerto: 5173 (dev)         │                             │  Puerto: 8080                │
└──────────────────────────────┘                             └──────────────────────────────┘
```

---

## Responsabilidades de cada parte

### Backend (zen-garden-server/) — Autoridad del juego

El servidor es la **fuente de verdad** de todo el estado del juego:

| Responsabilidad | Detalle |
|---|---|
| Conexiones WebSocket | Acepta, mantiene (ping/pong) y cierra conexiones |
| Creación de jugadores | Asigna ID, color, posición inicial aleatoria |
| Simulación de movimiento | Actualiza posiciones 20 veces/segundo |
| Detección de colisiones | Jugador vs luz (distancia euclidiana) |
| Crecimiento de jugadores | 30% del tamaño de la luz recolectada |
| Spawn de luces | 15 iniciales, luego cada 2-5s hasta máx. 40 |
| Ranking | Top 10 jugadores ordenados por tamaño |
| Broadcast de estado | Envía `state` a todos los jugadores cada tick |

### Frontend (zen-garden/) — Presentación e input

El cliente es un **renderizador puro** que no calcula física ni colisiones:

| Responsabilidad | Detalle |
|---|---|
| Pantalla de login | Formulario + animación de partículas de fondo |
| Conexión WebSocket | Conecta, envía join, maneja desconexión |
| Captura de input | Teclado (WASD/flechas) y mouse (click-to-move) |
| Renderizado Canvas 2D | Estrellas, grilla, luces, jugadores, UI |
| Cámara | Sigue al jugador local, culling de viewport |
| Efectos visuales | Glow, pulso sinusoidal, partículas al recolectar |
| UI overlay | Info del jugador, ranking, controles, minimapa |

---

## Protocolo de Comunicación

### Flujo de conexión

```
Cliente                                Servidor
  │                                       │
  │─── HTTP GET /ws ─────────────────────►│
  │◄── 101 Switching Protocols ──────────│  (WebSocket upgrade)
  │                                       │
  │─── { type: "join", name: "Ana" } ───►│  readPump() decodifica
  │                                       │  → Crea Player con ID, color, posición
  │◄── { type: "welcome",               │
  │      id: "p1",                        │
  │      mapW: 4000, mapH: 4000 } ──────│
  │                                       │
  │   ┌─── GAME LOOP (cada 50ms) ────┐  │
  │   │                               │  │
  │─── { type: "input", dx:1, dy:0 }►│  │  Jugador se mueve a la derecha
  │─── { type: "target", x, y } ────►│  │  Jugador va hacia el click
  │   │                               │  │
  │   │  tick():                       │  │
  │   │  ├── Update posiciones         │  │
  │   │  ├── Colisiones jugador-luz    │  │
  │   │  ├── Calcular ranking          │  │
  │   │  └── Serializar estado         │  │
  │   │                               │  │
  │◄── { type: "state",              │  │
  │      players: [...],               │  │
  │      lights: [...],                │  │
  │      top: [...] } ───────────────│  │
  │   │                               │  │
  │   └───────────────────────────────┘  │
  │                                       │
  │◄── Ping ─────────────────────────────│  (cada 54s keep-alive)
  │─── Pong ─────────────────────────────►│
```

### Formato de mensajes

| Dirección | Tipo | Campos | Frecuencia |
|---|---|---|---|
| C→S | `join` | `name` | 1 vez al conectar |
| C→S | `input` | `dx`, `dy` | Cada keydown/keyup |
| C→S | `target` | `x`, `y` | Cada click en canvas |
| S→C | `welcome` | `id`, `mapW`, `mapH` | 1 vez tras join |
| S→C | `state` | `players`, `lights`, `top` | 20 veces/segundo |

---

## Mecánicas del Juego

### Mapa
- Dimensiones: **4000 x 4000** unidades
- Fondo oscuro (#050515) con estrellas animadas
- Grilla visual de 100px
- Borde rojo semi-transparente

### Jugadores
- Tamaño inicial: **15 unidades**
- Velocidad base: **200 u/s** (decrece con el tamaño)
- Fórmula de velocidad: `200 / (1 + (size - 15) * 0.008)`
- Posición inicial: aleatoria (margen de 200 del borde)
- 16 colores posibles asignados aleatoriamente
- Clampeados dentro del mapa (no pueden salir)

### Luces (coleccionables)
- Tamaño: **6-16 unidades** (aleatorio)
- Spawn inicial: 15 luces
- Spawn continuo: cada 2-5 segundos
- Máximo en mapa: 40 luces simultáneas
- Al recolectar: jugador crece **30% del tamaño de la luz**

### Colisión
- Tipo: **círculo vs círculo**
- Condición: `distancia(jugador, luz) < jugador.size + luz.size`
- Solo jugador vs luz (no hay colisión jugador vs jugador)

### Ranking
- Top 10 jugadores por tamaño
- Actualizado cada tick (20 veces/segundo)
- Tamaño redondeado a 1 decimal

---

## Modelo de Concurrencia (Backend)

```
┌─────────────────────────────────────────────────────┐
│                   POR CLIENTE                        │
│                                                      │
│  goroutine: readPump()                              │
│  ├── Lee WebSocket continuamente                    │
│  ├── Decodifica JSON                                │
│  └── Envía a canales del Game:                      │
│      ├── game.join      (para "join")               │
│      └── player.Set*()  (para "input"/"target")     │
│                                                      │
│  goroutine: writePump()                             │
│  ├── Lee del canal client.send                      │
│  ├── Escribe al WebSocket                           │
│  └── Envía pings cada 54 segundos                   │
└─────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────┐
│              GOROUTINE ÚNICA: Game.Run()             │
│                                                      │
│  select {                                            │
│    case <-register:   // nuevo cliente conectado     │
│    case <-unregister: // cliente desconectado        │
│    case <-join:       // jugador se une al juego     │
│    case <-spawnTimer: // crear nueva luz             │
│    case <-ticker:     // tick() cada 50ms            │
│  }                                                   │
│                                                      │
│  Todo el estado del juego (players, lights, clients) │
│  se modifica SOLO en esta goroutine → sin locks      │
│  para datos del juego.                               │
│                                                      │
│  Player usa sync.RWMutex porque readPump() llama    │
│  a SetDirection/SetTarget desde otra goroutine.      │
└─────────────────────────────────────────────────────┘
```

---

## Pipeline de Renderizado (Frontend)

```
requestAnimationFrame → gameLoop()
│
├── updateParticles()
│   └── Física: velocidad *= 0.96, vida -= decay
│
└── drawGame()
    ├── Fondo #050515
    ├── 200 estrellas (brillo pulsante sinusoidal)
    ├── Cámara: ctx.translate(canvas/2 - me.x, canvas/2 - me.y)
    ├── Grilla 100px (opacidad 0.03)
    ├── Borde del mapa (rojo, opacidad 0.2)
    ├── Luces: gradiente radial + shadowBlur + core blanco
    ├── Partículas (al recolectar luces)
    ├── Jugadores (ordenados por tamaño, más pequeños primero):
    │   ├── Glow exterior (gradiente radial)
    │   ├── Cuerpo (círculo con color)
    │   ├── Highlight interior (reflejo blanco)
    │   ├── Anillo blanco (solo jugador local)
    │   └── Nombre (fuente Georgia, sobre el círculo)
    └── UI Overlay (sin transformación de cámara):
        ├── Nombre + tamaño (arriba-izquierda)
        ├── Ranking top 10 (arriba-derecha, fondo redondeado)
        ├── Hint de controles (abajo-centro, desaparece en 400 frames)
        └── Minimapa 140x140 (abajo-derecha)
```

---

## Detección de recolección en el cliente

El servidor no envía eventos de "luz recolectada". El cliente lo detecta por diferencia:

```javascript
// Al recibir nuevo state:
const newLightIds = new Set(msg.lights.map(l => l.id))
for (const oldLight of previousLights) {
    if (!newLightIds.has(oldLight.id)) {
        // Esta luz desapareció → generar 10 partículas
        spawnConsumeParticles(oldLight.x, oldLight.y)
    }
}
```

Partículas: 10 por luz, distribuidas en ángulos uniformes, con velocidad aleatoria, color HSL (amarillo-naranja), decaimiento gradual.

---

## Cómo ejecutar el proyecto completo

```bash
# Terminal 1: Backend
cd zen-garden-server
go run .
# → Servidor escuchando en :8080

# Terminal 2: Frontend
cd zen-garden
npm install
npm run dev
# → App disponible en http://localhost:5173
```

---

## Stack completo

| Capa | Tecnología | Archivo(s) clave |
|---|---|---|
| Servidor HTTP | Go `net/http` | main.go |
| WebSocket | Gorilla WebSocket | hub.go |
| Game engine | Go (goroutines + channels) | game.go |
| Entidad jugador | Go (sync.RWMutex) | player.go |
| Framework UI | Svelte 5 ($state) | App.svelte |
| Renderizado | HTML5 Canvas 2D | App.svelte |
| Build | Vite 7 | vite.config.js |
| Comunicación | WebSocket nativo (browser) | App.svelte |
