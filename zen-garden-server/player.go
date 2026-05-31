package main

import (
	"math"
	"math/rand"
	"sync"
)

var playerColors = []string{
	"#ff6b6b", "#ffd93d", "#6bcb77", "#4d96ff",
	"#ff6bcb", "#845ec2", "#ff9671", "#00c9a7",
	"#ffc75f", "#f9f871", "#c34a36", "#008b74",
	"#d65db1", "#0089ba", "#ff6f91", "#67e8f9",
}

type Player struct {
	mu    sync.RWMutex
	ID    string  `json:"id"`
	Name  string  `json:"name"`
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	Size  float64 `json:"size"`
	Color string  `json:"color"`

	// Movement
	dx      float64
	dy      float64
	targetX float64
	targetY float64
	hasTarget bool
}

func NewPlayer(id, name string, mapW, mapH float64, obstacles []*Obstacle) *Player {
	px, py := 200+rand.Float64()*(mapW-400), 200+rand.Float64()*(mapH-400)
	for attempts := 0; attempts < 50; attempts++ {
		inside := false
		for _, obs := range obstacles {
			dx := px - obs.X
			dy := py - obs.Y
			dist := math.Sqrt(dx*dx + dy*dy)
			if dist < 15+obs.Radius+10 {
				inside = true
				break
			}
		}
		if !inside {
			break
		}
		px = 200 + rand.Float64()*(mapW-400)
		py = 200 + rand.Float64()*(mapH-400)
	}
	return &Player{
		ID:    id,
		Name:  name,
		X:     px,
		Y:     py,
		Size:  15,
		Color: playerColors[rand.Intn(len(playerColors))],
	}
}

func (p *Player) SetDirection(dx, dy float64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.hasTarget = false
	// Normalize
	length := math.Sqrt(dx*dx + dy*dy)
	if length > 0 {
		p.dx = dx / length
		p.dy = dy / length
	} else {
		p.dx = 0
		p.dy = 0
	}
}

func (p *Player) SetTarget(x, y float64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.targetX = x
	p.targetY = y
	p.hasTarget = true
	p.dx = 0
	p.dy = 0
}

func (p *Player) Speed() float64 {
	// Bigger players are slightly slower
	return 200.0 / (1.0 + (p.Size-15)*0.008)
}

func (p *Player) Update(dt, mapW, mapH float64) {
	p.mu.Lock()
	defer p.mu.Unlock()

	speed := 200.0 / (1.0 + (p.Size-15)*0.008)

	if p.hasTarget {
		diffX := p.targetX - p.X
		diffY := p.targetY - p.Y
		dist := math.Sqrt(diffX*diffX + diffY*diffY)
		if dist < 3 {
			p.hasTarget = false
		} else {
			p.dx = diffX / dist
			p.dy = diffY / dist
		}
	}

	p.X += p.dx * speed * dt
	p.Y += p.dy * speed * dt

	// Clamp to map bounds
	if p.X < p.Size {
		p.X = p.Size
	}
	if p.Y < p.Size {
		p.Y = p.Size
	}
	if p.X > mapW-p.Size {
		p.X = mapW - p.Size
	}
	if p.Y > mapH-p.Size {
		p.Y = mapH - p.Size
	}
}

func (p *Player) DeflectFromObstacle(ox, oy, oRadius float64) {
	p.mu.Lock()
	defer p.mu.Unlock()

	dx := p.X - ox
	dy := p.Y - oy
	dist := math.Sqrt(dx*dx + dy*dy)
	minDist := p.Size + oRadius

	if dist >= minDist || dist == 0 {
		return
	}

	// Collision normal (obstacle center → player)
	nx := dx / dist
	ny := dy / dist

	// Push player outside
	overlap := minDist - dist
	p.X += nx * (overlap + 1)
	p.Y += ny * (overlap + 1)

	// Reflect velocity: v' = v - 2(v·n)n
	dot := p.dx*nx + p.dy*ny
	if dot < 0 {
		p.dx -= 2 * dot * nx
		p.dy -= 2 * dot * ny
		p.dx *= 0.8
		p.dy *= 0.8
	}

	p.hasTarget = false
}

func (p *Player) Grow(amount float64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.Size += amount
}

type PlayerState struct {
	ID    string  `json:"id"`
	Name  string  `json:"name"`
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	Size  float64 `json:"size"`
	Color string  `json:"color"`
}

func (p *Player) State() PlayerState {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return PlayerState{
		ID:    p.ID,
		Name:  p.Name,
		X:     p.X,
		Y:     p.Y,
		Size:  p.Size,
		Color: p.Color,
	}
}
