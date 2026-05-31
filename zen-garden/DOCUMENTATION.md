# Zen Garden - Documentación Frontend

## Descripción General

Zen Garden es el frontend de un juego multijugador en tiempo real estilo **Agar.io**, construido con **Svelte 5** y renderizado completamente en **HTML5 Canvas 2D**. No usa imágenes ni assets externos — todos los gráficos son generados proceduralmente.

---

## Estructura del Proyecto

```
zen-garden/
├── src/
│   ├── main.js          → Punto de entrada (monta App en #app)
│   ├── App.svelte       → Componente único con toda la lógica del juego
│   └── app.css          → Estilos globales (reset, body)
├── public/              → (vacío, sin assets estáticos)
├── index.html           → HTML base con <div id="app">
├── package.json         → Dependencias y scripts
├── vite.config.js       → Configuración de Vite
├── svelte.config.js     → Configuración de Svelte
└── jsconfig.json        → Configuración de JavaScript/TypeScript
```

### Stack Tecnológico

| Tecnología | Versión | Uso |
|---|---|---|
| Svelte | 5.45.2 | Framework reactivo (usa `$state`) |
| Vite | 7.3.1 | Bundler y servidor de desarrollo |
| Canvas 2D | Nativo | Renderizado del juego |
| WebSocket | Nativo | Comunicación en tiempo real |

### Scripts

```bash
npm run dev      # Servidor de desarrollo con HMR
npm run build    # Build de producción
npm run preview  # Preview del build
```

---

## Arquitectura

```
┌─────────────────────────────────────────────────────────┐
│                    index.html                            │
│                  <div id="app">                          │
└────────────────────────┬────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────┐
│                     main.js                              │
│              mount(App, { target })                      │
└────────────────────────┬────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────┐
│                   App.svelte                             │
│                                                          │
│  ┌────────────────┐    ┌─────────────────────────────┐  │
│  │ PANTALLA LOGIN  │    │     PANTALLA DE JUEGO       │  │
│  │                │    │                              │  │
│  │ Canvas fondo   │    │  Canvas principal            │  │
│  │ Partículas     │    │  ├── Estrellas de fondo      │  │
│  │ Formulario     │    │  ├── Grilla del mundo        │  │
│  │ nombre         │    │  ├── Borde del mapa          │  │
│  │                │    │  ├── Luces (coleccionables)   │  │
│  │                │    │  ├── Partículas               │  │
│  │                │    │  ├── Jugadores                │  │
│  │                │    │  └── UI Overlay               │  │
│  │                │    │      ├── Info jugador (↖)     │  │
│  │                │    │      ├── Ranking (↗)          │  │
│  │                │    │      ├── Controles (↓)        │  │
│  │                │    │      └── Minimapa (↘)         │  │
│  └────────────────┘    └─────────────────────────────┘  │
│                                                          │
│  ┌──────────────────────────────────────────────────┐   │
│  │              ESTADO ($state)                      │   │
│  │  screen, ws, myId, playerName                    │   │
│  │  players[], lights[], topPlayers[]               │   │
│  │  keys{}, particles[], bgStars[]                  │   │
│  │  canvas, ctx, time, animId                       │   │
│  └──────────────────────────────────────────────────┘   │
│                                                          │
│  ┌──────────────────────────────────────────────────┐   │
│  │           WEBSOCKET ↔ SERVIDOR                    │   │
│  │  ws://localhost:8080/ws                           │   │
│  └──────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────┘
```

---

## Pantallas

### Pantalla de Login

**Elementos visuales:**
- Canvas animado de fondo con partículas flotantes
- Título "Luces Errantes" con fuente Georgia (serif)
- Campo de texto para nombre del jugador (máx. 20 caracteres)
- Botón "Jugar" para conectarse
- Mensaje de error si la conexión falla

**Flujo:**
1. Usuario escribe su nombre
2. Presiona "Jugar" o Enter
3. Se abre conexión WebSocket
4. Se envía mensaje `join` con el nombre
5. Al recibir `welcome`, se cambia a pantalla de juego

### Pantalla de Juego

**Canvas a pantalla completa** renderizando:
1. Fondo oscuro (espacio)
2. 200 estrellas animadas con brillo pulsante
3. Grilla de 100px para referencia espacial
4. Borde visible del mapa
5. Luces coleccionables con efecto de brillo (gradiente radial)
6. Jugadores con sombras, glow y nombres
7. Partículas cuando se recolecta una luz
8. Overlay de UI (info, ranking, controles, minimapa)

---

## Estado de la Aplicación

Todo el estado se maneja con `$state` de Svelte 5 dentro de `App.svelte`:

```javascript
// Control de pantalla
let screen = $state('login')      // 'login' | 'game'

// Conexión
let ws = $state(null)             // WebSocket | null
let myId = $state('')             // ID asignado por el servidor
let playerName = $state('')       // Nombre del jugador

// Mapa
let mapW = $state(4000)           // Ancho del mapa
let mapH = $state(4000)           // Alto del mapa

// Entidades del juego
let players = $state([])          // Array de jugadores
let lights = $state([])           // Array de luces
let topPlayers = $state([])       // Top 10 ranking

// Renderizado
let canvas, ctx                   // Referencia al canvas
let time = $state(0)              // Contador de frames
let animId                        // ID de requestAnimationFrame

// Input
let keys = $state({})             // Teclas presionadas

// Efectos visuales
let particles = $state([])        // Partículas de recolección
let bgStars = $state([])          // Estrellas de fondo
let loginParticles = $state([])   // Partículas del login

// Errores
let errorMsg = $state('')         // Mensaje de error
```

### Estructuras de Datos

```javascript
// Jugador (recibido del servidor)
{
    id: "p1",
    name: "Alice",
    x: 500.0,
    y: 600.0,
    size: 25.5,
    color: "#ff6b6b"
}

// Luz (recibida del servidor)
{
    id: "l1",
    x: 1000.0,
    y: 1200.0,
    size: 8.5
}

// Partícula (solo cliente)
{
    x, y,          // Posición
    vx, vy,        // Velocidad
    r,             // Radio
    life,          // Vida (0-1)
    decay,         // Velocidad de decaimiento
    color          // Color de la partícula
}

// Estrella de fondo (solo cliente)
{
    x, y,          // Posición en pantalla
    r,             // Radio
    phase,         // Fase de animación
    speed          // Velocidad de pulso
}
```

---

## Sistema de Renderizado

### Pipeline de Renderizado (cada frame)

```
gameLoop() → requestAnimationFrame
│
├── 1. Limpiar canvas (fondo oscuro #0a0a12)
│
├── 2. Dibujar estrellas de fondo
│      └── 200 estrellas con brillo sinusoidal pulsante
│
├── 3. Aplicar cámara (centrada en jugador local)
│      └── ctx.translate(offsetX, offsetY)
│
├── 4. Dibujar grilla del mundo
│      └── Líneas cada 100px con opacidad sutil
│
├── 5. Dibujar borde del mapa
│      └── Rectángulo con stroke semi-transparente
│
├── 6. Dibujar luces
│      └── Gradiente radial + shadowBlur para efecto glow
│
├── 7. Actualizar y dibujar partículas
│      └── Física: velocidad, decaimiento, vida
│
├── 8. Dibujar jugadores
│      ├── Sombra y glow
│      ├── Círculo con color del jugador
│      └── Nombre sobre el círculo
│
├── 9. Restaurar cámara
│      └── ctx.restore()
│
├── 10. Dibujar UI overlay
│       ├── Info del jugador (arriba-izquierda)
│       ├── Ranking top 10 (arriba-derecha)
│       ├── Hint de controles (abajo-centro, se desvanece)
│       └── Minimapa 140x140px (abajo-derecha)
│
└── 11. Incrementar time, solicitar siguiente frame
```

### Sistema de Cámara

La cámara sigue al jugador local:

```javascript
// Encontrar jugador local
const me = players.find(p => p.id === myId)

// Calcular offset para centrar al jugador
const offsetX = canvas.width / 2 - me.x
const offsetY = canvas.height / 2 - me.y

// Aplicar transformación
ctx.save()
ctx.translate(offsetX, offsetY)
// ... dibujar mundo ...
ctx.restore()
```

**Culling**: Los objetos fuera del viewport no se dibujan (optimización).

### Efectos Visuales

| Efecto | Técnica | Aplicado a |
|---|---|---|
| Glow/Brillo | `shadowBlur` + `shadowColor` | Luces, jugadores |
| Pulso | `sin(time * speed + phase)` | Estrellas, luces |
| Gradiente | `createRadialGradient()` | Luces |
| Partículas | Física simple (velocidad + decaimiento) | Recolección de luces |
| Fade out | Opacidad basada en vida restante | Partículas, hint de controles |

### Minimapa

Renderizado en la esquina inferior derecha (140x140px):
- Fondo semi-transparente
- Todas las luces como puntos pequeños
- Todos los jugadores como puntos
- Jugador local resaltado en blanco

---

## Controles e Input

### Teclado

| Tecla | Acción |
|---|---|
| W / ArrowUp | Mover arriba |
| A / ArrowLeft | Mover izquierda |
| S / ArrowDown | Mover abajo |
| D / ArrowRight | Mover derecha |
| Enter | Enviar formulario de login |

**Procesamiento de input:**

```javascript
function sendInput() {
    let dx = 0, dy = 0
    if (keys['w'] || keys['arrowup'])    dy -= 1
    if (keys['s'] || keys['arrowdown'])  dy += 1
    if (keys['a'] || keys['arrowleft'])  dx -= 1
    if (keys['d'] || keys['arrowright']) dx += 1
    ws.send(JSON.stringify({ type: 'input', dx, dy }))
}
```

Se envía un mensaje `input` al servidor cada vez que se presiona o suelta una tecla.

### Mouse

**Click en el canvas** → Click-to-move:

```javascript
function handleCanvasClick(e) {
    // Convertir coordenadas de pantalla a coordenadas del mundo
    const worldX = e.clientX - offsetX
    const worldY = e.clientY - offsetY
    ws.send(JSON.stringify({ type: 'target', x: worldX, y: worldY }))
}
```

---

## Comunicación WebSocket

### Conexión

```javascript
const proto = location.protocol === 'https:' ? 'wss' : 'ws'
ws = new WebSocket(`${proto}://localhost:8080/ws`)
```

### Mensajes Enviados (Cliente → Servidor)

| Tipo | Payload | Cuándo |
|---|---|---|
| `join` | `{ type, name }` | Al presionar "Jugar" |
| `input` | `{ type, dx, dy }` | Cada keydown/keyup |
| `target` | `{ type, x, y }` | Click en el canvas |

### Mensajes Recibidos (Servidor → Cliente)

| Tipo | Payload | Acción |
|---|---|---|
| `welcome` | `{ type, id, mapW, mapH }` | Guardar ID, dimensiones del mapa, cambiar a pantalla de juego |
| `state` | `{ type, players, lights, top }` | Actualizar estado local, detectar luces recolectadas, generar partículas |

### Detección de Recolección de Luces (Cliente)

Cuando se recibe un `state`, el cliente compara las luces anteriores con las nuevas. Si una luz desapareció y un jugador está cerca, se generan partículas en esa posición:

```
Por cada luz que ya no está en el nuevo estado:
  → Buscar jugador cercano
  → Generar 10 partículas con colores amarillo-naranja
  → Física: velocidad aleatoria, decaimiento gradual
```

---

## Flujo Completo del Juego

```
1. Usuario abre la app
   └── Se muestra pantalla de login con animación de partículas

2. Usuario escribe nombre y presiona "Jugar"
   ├── Se abre WebSocket a ws://localhost:8080/ws
   ├── Se envía: { type: "join", name: "..." }
   └── Se espera respuesta

3. Servidor responde con "welcome"
   ├── Se guarda myId y dimensiones del mapa
   ├── Se cambia a pantalla de juego
   ├── Se generan 200 estrellas de fondo
   └── Se inicia gameLoop()

4. Loop de juego (cada frame via requestAnimationFrame)
   ├── Recibir estado del servidor (20 veces/seg)
   ├── Procesar input del teclado/mouse
   ├── Enviar input al servidor
   ├── Renderizar mundo completo en canvas
   └── Actualizar partículas y animaciones

5. Desconexión
   ├── WebSocket se cierra
   ├── Se muestra mensaje de error
   └── Se vuelve a pantalla de login
```

---

## Rendimiento

| Optimización | Descripción |
|---|---|
| Canvas único | Un solo contexto de renderizado 2D |
| Culling de viewport | Objetos fuera de pantalla no se dibujan |
| `requestAnimationFrame` | Sincronizado con refresh rate del monitor |
| Sin manipulación DOM | Todo renderizado via Canvas |
| Sin dependencias runtime | Solo Svelte (compilado, sin overhead) |
| Sin assets externos | Todo generado proceduralmente |

---

## Cómo Ejecutar

```bash
# Instalar dependencias
npm install

# Servidor de desarrollo
npm run dev

# Build de producción
npm run build
```

> **Nota**: Requiere que el servidor backend esté corriendo en `localhost:8080`.
