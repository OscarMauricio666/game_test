package main

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"sort"
	"time"
)

const (
	MapWidth  = 4000.0
	MapHeight = 4000.0
	TickRate  = 20 // ticks per second
	MaxLights = 40
)

type Light struct {
	ID   string  `json:"id"`
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
	Size float64 `json:"size"`
}

type Obstacle struct {
	ID     string  `json:"id"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Radius float64 `json:"radius"`
}

type JoinRequest struct {
	client *Client
	name   string
}

type RankEntry struct {
	Name string  `json:"name"`
	Size float64 `json:"size"`
}

type StateMessage struct {
	Type    string        `json:"type"`
	Players []PlayerState `json:"players"`
	Lights  []Light       `json:"lights"`
	Top     []RankEntry   `json:"top"`
}

type WelcomeMessage struct {
	Type      string     `json:"type"`
	ID        string     `json:"id"`
	MapW      float64    `json:"mapW"`
	MapH      float64    `json:"mapH"`
	Obstacles []Obstacle `json:"obstacles"`
}

type ChatRequest struct {
	sender string
	text   string
}

type ChatMessage struct {
	Type      string `json:"type"`
	Sender    string `json:"sender"`
	Text      string `json:"text"`
	Timestamp int64  `json:"timestamp"`
}

type Game struct {
	clients    map[*Client]bool
	register   chan *Client
	unregister chan *Client
	join       chan *JoinRequest
	chat       chan *ChatRequest

	players   map[string]*Player // keyed by ID
	lights    map[string]*Light
	obstacles []*Obstacle
	nextID    int
}

func NewGame() *Game {
	g := &Game{
		clients:    make(map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		join:       make(chan *JoinRequest),
		chat:       make(chan *ChatRequest, 64),
		players:    make(map[string]*Player),
		lights:     make(map[string]*Light),
	}
	g.generateObstacles()
	return g
}

func (g *Game) generateObstacles() {
	numObstacles := 15 + rand.Intn(11) // 15-25
	margin := 200.0
	attempts := 0
	for i := 0; i < numObstacles && attempts < 200; i++ {
		radius := 40 + rand.Float64()*80
		ox := margin + radius + rand.Float64()*(MapWidth-2*margin-2*radius)
		oy := margin + radius + rand.Float64()*(MapHeight-2*margin-2*radius)

		overlap := false
		for _, existing := range g.obstacles {
			dx := ox - existing.X
			dy := oy - existing.Y
			dist := math.Sqrt(dx*dx + dy*dy)
			if dist < radius+existing.Radius+50 {
				overlap = true
				break
			}
		}
		if overlap {
			attempts++
			i--
			continue
		}

		g.nextID++
		g.obstacles = append(g.obstacles, &Obstacle{
			ID:     fmt.Sprintf("o%d", g.nextID),
			X:      ox,
			Y:      oy,
			Radius: radius,
		})
	}
}

func (g *Game) obstacleList() []Obstacle {
	list := make([]Obstacle, len(g.obstacles))
	for i, o := range g.obstacles {
		list[i] = *o
	}
	return list
}

func (g *Game) isInsideObstacle(x, y, size float64) bool {
	for _, obs := range g.obstacles {
		dx := x - obs.X
		dy := y - obs.Y
		dist := math.Sqrt(dx*dx + dy*dy)
		if dist < size+obs.Radius {
			return true
		}
	}
	return false
}

func (g *Game) genID() string {
	g.nextID++
	return fmt.Sprintf("p%d", g.nextID)
}

func (g *Game) Run() {
	ticker := time.NewTicker(time.Second / TickRate)
	defer ticker.Stop()

	spawnTimer := time.NewTicker(time.Duration(2000+rand.Intn(2000)) * time.Millisecond)
	defer spawnTimer.Stop()

	// Spawn initial lights
	for i := 0; i < 15; i++ {
		g.spawnLight()
	}

	for {
		select {
		case client := <-g.register:
			g.clients[client] = true

		case client := <-g.unregister:
			if _, ok := g.clients[client]; ok {
				delete(g.clients, client)
				close(client.send)
				if client.player != nil {
					delete(g.players, client.player.ID)
				}
			}

		case req := <-g.join:
			id := g.genID()
			player := NewPlayer(id, req.name, MapWidth, MapHeight, g.obstacles)
			req.client.player = player
			g.players[id] = player

			welcome, _ := json.Marshal(WelcomeMessage{
				Type:      "welcome",
				ID:        id,
				MapW:      MapWidth,
				MapH:      MapHeight,
				Obstacles: g.obstacleList(),
			})
			select {
			case req.client.send <- welcome:
			default:
			}

		case chatReq := <-g.chat:
			chatMsg, _ := json.Marshal(ChatMessage{
				Type:      "chat",
				Sender:    chatReq.sender,
				Text:      chatReq.text,
				Timestamp: time.Now().UnixMilli(),
			})
			for client := range g.clients {
				if client.player != nil {
					select {
					case client.send <- chatMsg:
					default:
					}
				}
			}

		case <-spawnTimer.C:
			if len(g.lights) < MaxLights {
				g.spawnLight()
			}
			// Reset timer with random interval
			spawnTimer.Reset(time.Duration(2000+rand.Intn(3000)) * time.Millisecond)

		case <-ticker.C:
			g.tick()
		}
	}
}

func (g *Game) spawnLight() {
	g.nextID++
	id := fmt.Sprintf("l%d", g.nextID)
	size := 6 + rand.Float64()*10
	var lx, ly float64
	for attempts := 0; attempts < 50; attempts++ {
		lx = 50 + rand.Float64()*(MapWidth-100)
		ly = 50 + rand.Float64()*(MapHeight-100)
		if !g.isInsideObstacle(lx, ly, size) {
			break
		}
	}
	g.lights[id] = &Light{
		ID:   id,
		X:    lx,
		Y:    ly,
		Size: size,
	}
}

func (g *Game) tick() {
	dt := 1.0 / float64(TickRate)

	// Update players
	for _, p := range g.players {
		p.Update(dt, MapWidth, MapHeight)
	}

	// Check collisions: player vs obstacle
	for _, obs := range g.obstacles {
		for _, player := range g.players {
			player.DeflectFromObstacle(obs.X, obs.Y, obs.Radius)
		}
	}

	// Check collisions: player vs light
	for lightID, light := range g.lights {
		for _, player := range g.players {
			ps := player.State()
			dx := ps.X - light.X
			dy := ps.Y - light.Y
			dist := math.Sqrt(dx*dx + dy*dy)
			if dist < ps.Size+light.Size {
				// Consume light
				growth := light.Size * 0.3
				player.Grow(growth)
				delete(g.lights, lightID)
				break
			}
		}
	}

	// Build state
	playerStates := make([]PlayerState, 0, len(g.players))
	for _, p := range g.players {
		playerStates = append(playerStates, p.State())
	}

	lightList := make([]Light, 0, len(g.lights))
	for _, l := range g.lights {
		lightList = append(lightList, *l)
	}

	// Ranking
	sort.Slice(playerStates, func(i, j int) bool {
		return playerStates[i].Size > playerStates[j].Size
	})
	top := make([]RankEntry, 0, 10)
	for i, ps := range playerStates {
		if i >= 10 {
			break
		}
		top = append(top, RankEntry{Name: ps.Name, Size: math.Round(ps.Size*10) / 10})
	}

	msg, _ := json.Marshal(StateMessage{
		Type:    "state",
		Players: playerStates,
		Lights:  lightList,
		Top:     top,
	})

	// Broadcast
	for client := range g.clients {
		if client.player != nil {
			select {
			case client.send <- msg:
			default:
			}
		}
	}
}
