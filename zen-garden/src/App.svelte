<script lang="ts">
  import { onMount } from 'svelte';

  // --- Types ---
  interface PlayerData {
    id: string;
    name: string;
    x: number;
    y: number;
    size: number;
    color: string;
  }

  interface LightData {
    id: string;
    x: number;
    y: number;
    size: number;
  }

  interface ObstacleData {
    id: string;
    x: number;
    y: number;
    radius: number;
  }

  interface RankEntry {
    name: string;
    size: number;
  }

  interface Particle {
    x: number;
    y: number;
    vx: number;
    vy: number;
    r: number;
    life: number;
    decay: number;
    color: string;
  }

  interface Star {
    x: number;
    y: number;
    r: number;
    phase: number;
    speed: number;
  }

  interface LoginParticle {
    x: number;
    y: number;
    r: number;
    vx: number;
    vy: number;
    phase: number;
  }

  interface ChatMsg {
    sender: string;
    text: string;
    timestamp: number;
  }

  interface Toast {
    id: number;
    text: string;
  }

  type WelcomeMsg = {
    type: 'welcome';
    id: string;
    mapW: number;
    mapH: number;
    obstacles: ObstacleData[];
  };

  type StateMsg = {
    type: 'state';
    players: PlayerData[];
    lights: LightData[];
    top: RankEntry[];
  };

  type ChatServerMsg = {
    type: 'chat';
    sender: string;
    text: string;
    timestamp: number;
  };

  type ServerMsg = WelcomeMsg | StateMsg | ChatServerMsg;

  // --- State ---
  let screen = $state<'login' | 'game'>('login');
  let playerName = $state('');
  let errorMsg = $state('');

  // WebSocket
  let ws = $state<WebSocket | null>(null);
  let myId = $state('');
  let mapW = $state(4000);
  let mapH = $state(4000);

  // Game state from server
  let players = $state<PlayerData[]>([]);
  let lights = $state<LightData[]>([]);
  let topPlayers = $state<RankEntry[]>([]);
  let obstacles = $state<ObstacleData[]>([]);

  // Chat
  let chatMessages = $state<ChatMsg[]>([]);
  let chatInput = $state('');
  let toasts = $state<Toast[]>([]);
  let nextToastId = 0;

  // Canvas
  let canvas: HTMLCanvasElement;
  let ctx: CanvasRenderingContext2D;
  let animId: number;
  let time = 0;

  // Input
  let keys: Record<string, boolean> = {};
  let particles: Particle[] = [];
  let bgStars: Star[] = [];

  // Login
  let loginParticles: LoginParticle[] = [];
  let loginCanvas: HTMLCanvasElement;
  let loginCtx: CanvasRenderingContext2D;
  let loginAnimId: number;

  // Performance: offscreen canvas cache for obstacles
  let obstacleCache: HTMLCanvasElement | null = null;
  let obstacleCacheX = 0;
  let obstacleCacheY = 0;
  let obstacleCacheW = 0;
  let obstacleCacheH = 0;

  // Client-side interpolation state
  let interpMap: Map<string, { prevX: number; prevY: number; targetX: number; targetY: number; renderX: number; renderY: number }> = new Map();
  let lastServerTime = 0;
  let interpProgress = 0;
  const SERVER_TICK_MS = 50; // server sends at 20fps = 50ms per tick

  const TWO_PI = Math.PI * 2;

  onMount(() => {
    initLoginAnimation();
    return () => {
      cancelAnimationFrame(loginAnimId);
      cancelAnimationFrame(animId);
      if (ws) ws.close();
    };
  });

  // --- LOGIN ANIMATION ---
  function initLoginAnimation(): void {
    if (!loginCanvas) return;
    loginCtx = loginCanvas.getContext('2d')!;
    loginCanvas.width = window.innerWidth;
    loginCanvas.height = window.innerHeight;

    loginParticles = [];
    for (let i = 0; i < 80; i++) {
      loginParticles.push({
        x: Math.random() * loginCanvas.width,
        y: Math.random() * loginCanvas.height,
        r: Math.random() * 2 + 0.5,
        vx: (Math.random() - 0.5) * 0.3,
        vy: (Math.random() - 0.5) * 0.3,
        phase: Math.random() * TWO_PI,
      });
    }
    loginLoop();
  }

  function loginLoop(): void {
    if (screen !== 'login') return;
    time++;
    const w = loginCanvas.width;
    const h = loginCanvas.height;
    loginCtx.fillStyle = '#050515';
    loginCtx.fillRect(0, 0, w, h);

    for (const p of loginParticles) {
      p.x += p.vx;
      p.y += p.vy;
      if (p.x < 0) p.x = w;
      if (p.x > w) p.x = 0;
      if (p.y < 0) p.y = h;
      if (p.y > h) p.y = 0;
      const glow = Math.sin(time * 0.02 + p.phase) * 0.3 + 0.7;
      loginCtx.globalAlpha = glow * 0.6;
      const grad = loginCtx.createRadialGradient(p.x, p.y, 0, p.x, p.y, p.r * 4);
      grad.addColorStop(0, 'rgba(255, 220, 80, 0.4)');
      grad.addColorStop(1, 'rgba(255, 220, 80, 0)');
      loginCtx.fillStyle = grad;
      loginCtx.beginPath();
      loginCtx.arc(p.x, p.y, p.r * 4, 0, TWO_PI);
      loginCtx.fill();
      loginCtx.fillStyle = '#ffe47a';
      loginCtx.beginPath();
      loginCtx.arc(p.x, p.y, p.r, 0, TWO_PI);
      loginCtx.fill();
    }
    loginCtx.globalAlpha = 1;
    loginAnimId = requestAnimationFrame(loginLoop);
  }

  // --- CONNECT ---
  function joinGame(): void {
    const name = playerName.trim();
    if (!name) {
      errorMsg = 'Escribe tu nombre';
      return;
    }
    errorMsg = '';

    const protocol = window.location.protocol === 'https:' ? 'wss' : 'ws';
    ws = new WebSocket(`${protocol}://${window.location.host}/ws`);

    ws.onopen = () => {
      ws!.send(JSON.stringify({ type: 'join', name }));
    };

    ws.onmessage = (e: MessageEvent) => {
      const msg: ServerMsg = JSON.parse(e.data);
      if (msg.type === 'welcome') {
        myId = msg.id;
        mapW = msg.mapW;
        mapH = msg.mapH;
        obstacles = msg.obstacles || [];
        obstacleCache = null; // invalidate cache
        screen = 'game';
        cancelAnimationFrame(loginAnimId);
        requestAnimationFrame(() => initGame());
      } else if (msg.type === 'chat') {
        chatMessages = [...chatMessages.slice(-49), {
          sender: msg.sender,
          text: msg.text,
          timestamp: msg.timestamp,
        }];
        const me = players.find(p => p.id === myId);
        if (me && msg.sender !== me.name) {
          const mentionRe = new RegExp('@' + me.name + '\\b');
          if (mentionRe.test(msg.text)) {
            addToast(`${msg.sender} te mencionó: "${msg.text}"`);
          }
        }
      } else if (msg.type === 'state') {
        if (lights.length > 0) {
          const newLightIds = new Set(msg.lights.map(l => l.id));
          for (const oldLight of lights) {
            if (!newLightIds.has(oldLight.id)) {
              spawnConsumeParticles(oldLight.x, oldLight.y);
            }
          }
        }

        // Update interpolation: shift current target → prev, set new target
        const now = performance.now();
        for (const p of msg.players) {
          const existing = interpMap.get(p.id);
          if (existing) {
            existing.prevX = existing.renderX;
            existing.prevY = existing.renderY;
            existing.targetX = p.x;
            existing.targetY = p.y;
          } else {
            interpMap.set(p.id, {
              prevX: p.x, prevY: p.y,
              targetX: p.x, targetY: p.y,
              renderX: p.x, renderY: p.y,
            });
          }
        }
        // Clean up disconnected players
        const activeIds = new Set(msg.players.map(p => p.id));
        for (const id of interpMap.keys()) {
          if (!activeIds.has(id)) interpMap.delete(id);
        }
        lastServerTime = now;
        interpProgress = 0;

        players = msg.players;
        lights = msg.lights;
        topPlayers = msg.top;
      }
    };

    ws.onclose = () => {
      if (screen === 'game') {
        screen = 'login';
        errorMsg = 'Conexión perdida. Intenta de nuevo.';
        cancelAnimationFrame(animId);
        requestAnimationFrame(() => initLoginAnimation());
      }
    };

    ws.onerror = () => {
      errorMsg = 'No se pudo conectar al servidor';
    };
  }

  function spawnConsumeParticles(x: number, y: number): void {
    for (let i = 0; i < 8; i++) {
      const angle = (i / 8) * TWO_PI;
      particles.push({
        x, y,
        vx: Math.cos(angle) * (Math.random() * 2 + 1),
        vy: Math.sin(angle) * (Math.random() * 2 + 1),
        r: Math.random() * 3 + 1,
        life: 1,
        decay: 0.02 + Math.random() * 0.01,
        color: `hsl(${40 + Math.random() * 30}, 100%, ${60 + Math.random() * 20}%)`,
      });
    }
  }

  // --- GAME ---
  function initGame(): void {
    if (!canvas) return;
    ctx = canvas.getContext('2d')!;
    canvas.width = window.innerWidth;
    canvas.height = window.innerHeight;

    bgStars = [];
    for (let i = 0; i < 150; i++) {
      bgStars.push({
        x: Math.random() * mapW,
        y: Math.random() * mapH,
        r: Math.random() * 1.5 + 0.3,
        phase: Math.random() * TWO_PI,
        speed: Math.random() * 0.015 + 0.003,
      });
    }

    window.addEventListener('resize', onResize);
    window.addEventListener('keydown', onKeyDown);
    window.addEventListener('keyup', onKeyUp);

    gameLoop();
  }

  function onResize(): void {
    if (canvas) {
      canvas.width = window.innerWidth;
      canvas.height = window.innerHeight;
      obstacleCache = null; // invalidate on resize
    }
  }

  function sendChat(): void {
    if (!ws || ws.readyState !== WebSocket.OPEN) return;
    const text = chatInput.trim();
    if (!text) return;
    ws.send(JSON.stringify({ type: 'chat', text }));
    chatInput = '';
  }

  function addToast(text: string): void {
    const id = nextToastId++;
    toasts = [...toasts, { id, text }];
    setTimeout(() => {
      toasts = toasts.filter(t => t.id !== id);
    }, 4000);
  }

  function onKeyDown(e: KeyboardEvent): void {
    if (document.activeElement && document.activeElement.tagName === 'INPUT') return;
    keys[e.key] = true;
    if (['ArrowUp', 'ArrowDown', 'ArrowLeft', 'ArrowRight', ' '].includes(e.key)) {
      e.preventDefault();
    }
    sendInput();
  }

  function onKeyUp(e: KeyboardEvent): void {
    if (document.activeElement && document.activeElement.tagName === 'INPUT') return;
    delete keys[e.key];
    sendInput();
  }

  function sendInput(): void {
    if (!ws || ws.readyState !== WebSocket.OPEN) return;
    let dx = 0;
    let dy = 0;
    if (keys['w'] || keys['W'] || keys['ArrowUp']) dy -= 1;
    if (keys['s'] || keys['S'] || keys['ArrowDown']) dy += 1;
    if (keys['a'] || keys['A'] || keys['ArrowLeft']) dx -= 1;
    if (keys['d'] || keys['D'] || keys['ArrowRight']) dx += 1;
    ws.send(JSON.stringify({ type: 'input', dx, dy }));
  }

  function handleCanvasClick(e: MouseEvent): void {
    if (!ws || ws.readyState !== WebSocket.OPEN) return;
    const me = players.find(p => p.id === myId);
    if (!me) return;

    const cw = canvas.width;
    const ch = canvas.height;
    const mePos = getPlayerRenderPos(me);
    const camX = mePos.rx - cw / 2;
    const camY = mePos.ry - ch / 2;

    ws.send(JSON.stringify({ type: 'target', x: e.clientX + camX, y: e.clientY + camY }));
  }

  function gameLoop(): void {
    time++;
    updateInterpolation();
    updateParticles();
    drawGame();
    animId = requestAnimationFrame(gameLoop);
  }

  function updateInterpolation(): void {
    const now = performance.now();
    const elapsed = now - lastServerTime;
    // Clamp t between 0 and 1 (allow slight overshoot to reach target)
    interpProgress = Math.min(elapsed / SERVER_TICK_MS, 1);

    for (const [id, interp] of interpMap) {
      interp.renderX = interp.prevX + (interp.targetX - interp.prevX) * interpProgress;
      interp.renderY = interp.prevY + (interp.targetY - interp.prevY) * interpProgress;
    }
  }

  function getPlayerRenderPos(p: PlayerData): { rx: number; ry: number } {
    const interp = interpMap.get(p.id);
    if (interp) {
      return { rx: interp.renderX, ry: interp.renderY };
    }
    return { rx: p.x, ry: p.y };
  }

  function updateParticles(): void {
    let writeIdx = 0;
    for (let i = 0; i < particles.length; i++) {
      const p = particles[i];
      p.x += p.vx;
      p.y += p.vy;
      p.vx *= 0.96;
      p.vy *= 0.96;
      p.life -= p.decay;
      if (p.life > 0) {
        particles[writeIdx++] = p;
      }
    }
    particles.length = writeIdx;
  }

  // Build obstacle offscreen cache
  function buildObstacleCache(camX: number, camY: number, cw: number, ch: number): void {
    const margin = 200;
    const oc = document.createElement('canvas');
    oc.width = cw + margin * 2;
    oc.height = ch + margin * 2;
    const octx = oc.getContext('2d')!;

    const ox0 = camX - margin;
    const oy0 = camY - margin;

    for (const obs of obstacles) {
      const ox = obs.x - ox0;
      const oy = obs.y - oy0;
      if (ox < -obs.radius - 50 || ox > oc.width + obs.radius + 50 ||
          oy < -obs.radius - 50 || oy > oc.height + obs.radius + 50) continue;

      const r = obs.radius;
      const segments = 12;
      octx.beginPath();
      for (let i = 0; i <= segments; i++) {
        const angle = (i / segments) * TWO_PI;
        const seed = obs.x * 7.3 + obs.y * 13.7 + i * 31.1;
        const jitter = 0.82 + 0.22 * Math.sin(seed);
        const px = ox + Math.cos(angle) * r * jitter;
        const py = oy + Math.sin(angle) * r * jitter;
        if (i === 0) octx.moveTo(px, py);
        else octx.lineTo(px, py);
      }
      octx.closePath();

      const grad = octx.createRadialGradient(ox, oy, 0, ox, oy, r);
      grad.addColorStop(0, 'rgba(80, 70, 60, 0.6)');
      grad.addColorStop(0.7, 'rgba(50, 45, 40, 0.5)');
      grad.addColorStop(1, 'rgba(30, 25, 20, 0.3)');
      octx.fillStyle = grad;
      octx.fill();

      octx.strokeStyle = 'rgba(120, 100, 80, 0.25)';
      octx.lineWidth = 1.5;
      octx.stroke();
    }

    obstacleCache = oc;
    obstacleCacheX = ox0;
    obstacleCacheY = oy0;
    obstacleCacheW = oc.width;
    obstacleCacheH = oc.height;
  }

  function drawGame(): void {
    if (!ctx) return;
    const cw = canvas.width;
    const ch = canvas.height;

    const me = players.find(p => p.id === myId);
    if (!me) {
      ctx.fillStyle = '#050515';
      ctx.fillRect(0, 0, cw, ch);
      ctx.fillStyle = 'rgba(255,255,255,0.5)';
      ctx.font = '18px Georgia';
      ctx.textAlign = 'center';
      ctx.fillText('Conectando...', cw / 2, ch / 2);
      return;
    }

    const mePos = getPlayerRenderPos(me);
    const camX = mePos.rx - cw / 2;
    const camY = mePos.ry - ch / 2;

    // Background
    ctx.fillStyle = '#050515';
    ctx.fillRect(0, 0, cw, ch);

    // Stars - batched into 3 brightness groups to minimize state changes
    ctx.fillStyle = '#fff';
    const starGroups: [number, Star[]][] = [[0.3, []], [0.4, []], [0.5, []]];
    for (const s of bgStars) {
      const sx = s.x - camX;
      const sy = s.y - camY;
      if (sx < -10 || sx > cw + 10 || sy < -10 || sy > ch + 10) continue;
      const brightness = Math.sin(time * s.speed + s.phase) * 0.3 + 0.7;
      const alpha = brightness * 0.5;
      // Bucket into 3 groups: dim (<0.35), mid (0.35-0.45), bright (>0.45)
      const idx = alpha < 0.35 ? 0 : alpha < 0.45 ? 1 : 2;
      starGroups[idx][1].push(s);
    }
    for (const [alpha, group] of starGroups) {
      if (group.length === 0) continue;
      ctx.globalAlpha = alpha;
      ctx.beginPath();
      for (const s of group) {
        const sx = s.x - camX;
        const sy = s.y - camY;
        ctx.moveTo(sx + s.r, sy);
        ctx.arc(sx, sy, s.r, 0, TWO_PI);
      }
      ctx.fill();
    }
    ctx.globalAlpha = 1;

    // Animated sinusoidal waves (step=8px for performance)
    ctx.strokeStyle = 'rgba(255, 255, 255, 0.025)';
    ctx.lineWidth = 1;
    const waveSpacing = 120;
    const waveAmp = 12;
    const waveFreq = 0.012;
    const waveSpeed = 0.025;

    const startYW = Math.floor(camY / waveSpacing) * waveSpacing;
    for (let wy = startYW; wy < camY + ch + waveSpacing; wy += waveSpacing) {
      ctx.beginPath();
      const baseY = wy - camY;
      const phaseY = time * waveSpeed + wy * 0.01;
      for (let sx = 0; sx <= cw; sx += 8) {
        const offy = Math.sin((sx + camX) * waveFreq + phaseY) * waveAmp;
        if (sx === 0) ctx.moveTo(sx, baseY + offy);
        else ctx.lineTo(sx, baseY + offy);
      }
      ctx.stroke();
    }

    const startXW = Math.floor(camX / waveSpacing) * waveSpacing;
    for (let wxl = startXW; wxl < camX + cw + waveSpacing; wxl += waveSpacing) {
      ctx.beginPath();
      const baseX = wxl - camX;
      const phaseX = time * waveSpeed + wxl * 0.01;
      for (let sy = 0; sy <= ch; sy += 8) {
        const offx = Math.sin((sy + camY) * waveFreq + phaseX) * waveAmp;
        if (sy === 0) ctx.moveTo(baseX + offx, sy);
        else ctx.lineTo(baseX + offx, sy);
      }
      ctx.stroke();
    }

    // Map border
    ctx.strokeStyle = 'rgba(255, 100, 100, 0.2)';
    ctx.lineWidth = 2;
    ctx.strokeRect(-camX, -camY, mapW, mapH);

    // Obstacles (cached offscreen canvas, rebuilt when camera moves far enough)
    if (obstacles.length > 0) {
      const needsRebuild = !obstacleCache ||
        Math.abs(camX - obstacleCacheX - 200) > 100 ||
        Math.abs(camY - obstacleCacheY - 200) > 100 ||
        cw + 400 !== obstacleCacheW ||
        ch + 400 !== obstacleCacheH;

      if (needsRebuild) {
        buildObstacleCache(camX, camY, cw, ch);
      }

      if (obstacleCache) {
        ctx.drawImage(obstacleCache, obstacleCacheX - camX, obstacleCacheY - camY);
      }
    }

    // Lights (NO shadowBlur - use only radial gradient for glow effect)
    for (const light of lights) {
      const lx = light.x - camX;
      const ly = light.y - camY;
      if (lx < -50 || lx > cw + 50 || ly < -50 || ly > ch + 50) continue;

      const pulse = Math.sin(time * 0.04 + light.x * 0.01) * 0.3 + 0.7;
      const lr = light.size;

      // Outer glow (gradient only, no shadowBlur)
      const glow = ctx.createRadialGradient(lx, ly, 0, lx, ly, lr * 4);
      glow.addColorStop(0, `rgba(255, 230, 100, ${pulse * 0.3})`);
      glow.addColorStop(0.5, `rgba(255, 220, 80, ${pulse * 0.08})`);
      glow.addColorStop(1, 'rgba(255, 220, 80, 0)');
      ctx.fillStyle = glow;
      ctx.beginPath();
      ctx.arc(lx, ly, lr * 4, 0, TWO_PI);
      ctx.fill();

      // Inner light
      ctx.globalAlpha = pulse;
      ctx.fillStyle = '#ffe680';
      ctx.beginPath();
      ctx.arc(lx, ly, lr, 0, TWO_PI);
      ctx.fill();

      // Core
      ctx.fillStyle = '#fff';
      ctx.beginPath();
      ctx.arc(lx, ly, lr * 0.4, 0, TWO_PI);
      ctx.fill();
      ctx.globalAlpha = 1;
    }

    // Particles
    for (const p of particles) {
      const px = p.x - camX;
      const py = p.y - camY;
      ctx.globalAlpha = p.life;
      ctx.fillStyle = p.color;
      ctx.beginPath();
      ctx.arc(px, py, p.r * p.life, 0, TWO_PI);
      ctx.fill();
    }
    ctx.globalAlpha = 1;

    // Players (NO shadowBlur - use gradient glow only)
    const sortedPlayers = [...players].sort((a, b) => a.size - b.size);
    for (const p of sortedPlayers) {
      const pos = getPlayerRenderPos(p);
      const px = pos.rx - camX;
      const py = pos.ry - camY;
      if (px < -100 || px > cw + 100 || py < -100 || py > ch + 100) continue;

      const isMe = p.id === myId;
      const glowPulse = Math.sin(time * 0.02 + p.x * 0.005) * 0.15 + 0.85;

      // Outer glow (gradient, no shadow)
      const glowR = p.size * 2.5;
      const glow = ctx.createRadialGradient(px, py, p.size * 0.5, px, py, glowR);
      glow.addColorStop(0, p.color + '30');
      glow.addColorStop(0.6, p.color + '10');
      glow.addColorStop(1, p.color + '00');
      ctx.fillStyle = glow;
      ctx.beginPath();
      ctx.arc(px, py, glowR, 0, TWO_PI);
      ctx.fill();

      // Body
      ctx.globalAlpha = glowPulse;
      ctx.fillStyle = p.color;
      ctx.beginPath();
      ctx.arc(px, py, p.size, 0, TWO_PI);
      ctx.fill();

      // Inner highlight
      ctx.fillStyle = 'rgba(255,255,255,0.2)';
      ctx.beginPath();
      ctx.arc(px - p.size * 0.25, py - p.size * 0.25, p.size * 0.4, 0, TWO_PI);
      ctx.fill();
      ctx.globalAlpha = 1;

      // Ring for local player
      if (isMe) {
        ctx.strokeStyle = 'rgba(255, 255, 255, 0.3)';
        ctx.lineWidth = 1.5;
        ctx.beginPath();
        ctx.arc(px, py, p.size + 4, 0, TWO_PI);
        ctx.stroke();
      }

      // Name
      ctx.fillStyle = isMe ? '#fff' : 'rgba(255,255,255,0.7)';
      ctx.font = `${Math.max(11, Math.min(14, p.size * 0.6))}px Georgia`;
      ctx.textAlign = 'center';
      ctx.fillText(p.name, px, py - p.size - 8);
    }

    // --- UI OVERLAY ---
    ctx.fillStyle = 'rgba(255,255,255,0.7)';
    ctx.font = '16px Georgia';
    ctx.textAlign = 'left';
    ctx.fillText(`${me.name}  |  ${Math.round(me.size * 10) / 10}`, 20, 30);

    // Ranking
    if (topPlayers.length > 0) {
      const rx = cw - 170;
      const ry = 20;
      ctx.fillStyle = 'rgba(0, 0, 0, 0.3)';
      ctx.beginPath();
      ctx.roundRect(rx - 10, ry - 5, 170, topPlayers.length * 22 + 30, 8);
      ctx.fill();

      ctx.fillStyle = 'rgba(255, 220, 80, 0.8)';
      ctx.font = 'bold 13px Georgia';
      ctx.textAlign = 'left';
      ctx.fillText('Ranking', rx, ry + 14);

      ctx.font = '12px Georgia';
      for (let i = 0; i < topPlayers.length; i++) {
        const entry = topPlayers[i];
        const isMyEntry = entry.name === me.name;
        ctx.fillStyle = isMyEntry ? 'rgba(255, 220, 80, 0.9)' : 'rgba(255,255,255,0.6)';
        ctx.fillText(`${i + 1}. ${entry.name}`, rx, ry + 36 + i * 22);
        ctx.textAlign = 'right';
        ctx.fillText(`${entry.size}`, rx + 150, ry + 36 + i * 22);
        ctx.textAlign = 'left';
      }
    }

    // Controls hint
    if (time < 400) {
      ctx.globalAlpha = Math.max(0, 1 - time / 400);
      ctx.fillStyle = 'rgba(255,255,255,0.5)';
      ctx.font = '14px Georgia';
      ctx.textAlign = 'center';
      ctx.fillText('WASD o flechas para moverte  |  Click para ir a un punto', cw / 2, ch - 30);
      ctx.globalAlpha = 1;
    }

    // Mini-map
    const mmW = 140;
    const mmH = 140;
    const mmX = cw - mmW - 15;
    const mmY = ch - mmH - 15;
    ctx.fillStyle = 'rgba(0, 0, 0, 0.35)';
    ctx.beginPath();
    ctx.roundRect(mmX, mmY, mmW, mmH, 6);
    ctx.fill();
    ctx.strokeStyle = 'rgba(255,255,255,0.15)';
    ctx.lineWidth = 1;
    ctx.beginPath();
    ctx.roundRect(mmX, mmY, mmW, mmH, 6);
    ctx.stroke();

    // Obstacles on mini-map
    for (const obs of obstacles) {
      const omx = mmX + (obs.x / mapW) * mmW;
      const omy = mmY + (obs.y / mapH) * mmH;
      const omr = Math.max(2, (obs.radius / mapW) * mmW);
      ctx.fillStyle = 'rgba(120, 100, 80, 0.5)';
      ctx.beginPath();
      ctx.arc(omx, omy, omr, 0, TWO_PI);
      ctx.fill();
    }

    // Lights on mini-map
    for (const light of lights) {
      const lmx = mmX + (light.x / mapW) * mmW;
      const lmy = mmY + (light.y / mapH) * mmH;
      ctx.fillStyle = 'rgba(255, 220, 80, 0.6)';
      ctx.beginPath();
      ctx.arc(lmx, lmy, 1.5, 0, TWO_PI);
      ctx.fill();
    }

    // Players on mini-map
    for (const p of players) {
      const pPos = getPlayerRenderPos(p);
      const pmx = mmX + (pPos.rx / mapW) * mmW;
      const pmy = mmY + (pPos.ry / mapH) * mmH;
      const isMe = p.id === myId;
      ctx.fillStyle = isMe ? '#fff' : p.color;
      ctx.beginPath();
      ctx.arc(pmx, pmy, isMe ? 3 : 2, 0, TWO_PI);
      ctx.fill();
    }
  }

  function handleKeypress(e: KeyboardEvent): void {
    if (e.key === 'Enter') joinGame();
  }
</script>

{#if screen === 'login'}
  <canvas class="login-bg" bind:this={loginCanvas}></canvas>
  <div class="login-container">
    <h1 class="game-title">Luces Errantes</h1>
    <p class="game-subtitle">Deambula por el mapa, encuentra luces y crece</p>
    <div class="login-form">
      <input
        type="text"
        placeholder="Tu nombre"
        maxlength="20"
        bind:value={playerName}
        onkeypress={handleKeypress}
      />
      <button onclick={joinGame}>Jugar</button>
    </div>
    {#if errorMsg}
      <p class="error">{errorMsg}</p>
    {/if}
  </div>
{:else}
  <canvas
    class="game-canvas"
    bind:this={canvas}
    onclick={handleCanvasClick}
  ></canvas>

  <!-- Chat UI -->
  <div class="chat-container">
    <div class="chat-messages">
      {#each chatMessages as msg}
        <div class="chat-msg">
          <span class="chat-sender">{msg.sender}:</span>
          <span class="chat-text">{@html msg.text.replace(/@(\w+)/g, '<span class="chat-mention">@$1</span>')}</span>
        </div>
      {/each}
    </div>
    <div class="chat-input-row">
      <input
        type="text"
        class="chat-input"
        placeholder="Mensaje... (@nombre para mencionar)"
        maxlength="200"
        bind:value={chatInput}
        onkeypress={(e) => { if (e.key === 'Enter') sendChat(); }}
      />
      <button class="chat-send" onclick={sendChat}>Enviar</button>
    </div>
  </div>

  <!-- Toast notifications -->
  {#each toasts as toast, i (toast.id)}
    <div class="toast" style="top: {60 + i * 55}px">
      {toast.text}
    </div>
  {/each}
{/if}

<style>
  .login-bg {
    position: fixed;
    top: 0;
    left: 0;
    width: 100%;
    height: 100%;
    z-index: 0;
  }

  .login-container {
    position: fixed;
    top: 50%;
    left: 50%;
    transform: translate(-50%, -50%);
    z-index: 10;
    text-align: center;
  }

  .game-title {
    font-size: 3rem;
    color: #ffe680;
    margin: 0 0 8px 0;
    font-weight: normal;
    letter-spacing: 0.15em;
    text-shadow: 0 0 40px rgba(255, 220, 80, 0.3);
  }

  .game-subtitle {
    color: rgba(255, 255, 255, 0.4);
    font-size: 1rem;
    margin: 0 0 40px 0;
  }

  .login-form {
    display: flex;
    gap: 12px;
    justify-content: center;
    flex-wrap: wrap;
  }

  .login-form input {
    background: rgba(255, 255, 255, 0.08);
    border: 1px solid rgba(255, 255, 255, 0.15);
    border-radius: 8px;
    padding: 12px 20px;
    font-size: 1.1rem;
    color: #fff;
    font-family: 'Georgia', serif;
    outline: none;
    width: 220px;
    transition: border-color 0.3s;
  }

  .login-form input:focus {
    border-color: rgba(255, 220, 80, 0.5);
  }

  .login-form input::placeholder {
    color: rgba(255, 255, 255, 0.3);
  }

  .login-form button {
    background: rgba(255, 220, 80, 0.15);
    border: 1px solid rgba(255, 220, 80, 0.3);
    border-radius: 8px;
    padding: 12px 32px;
    font-size: 1.1rem;
    color: #ffe680;
    font-family: 'Georgia', serif;
    cursor: pointer;
    transition: all 0.3s;
  }

  .login-form button:hover {
    background: rgba(255, 220, 80, 0.25);
    border-color: rgba(255, 220, 80, 0.5);
  }

  .error {
    color: #ff6b6b;
    margin-top: 16px;
    font-size: 0.9rem;
  }

  .game-canvas {
    position: fixed;
    top: 0;
    left: 0;
    width: 100%;
    height: 100%;
    cursor: crosshair;
    display: block;
  }

  .chat-container {
    position: fixed;
    bottom: 10px;
    left: 10px;
    width: 350px;
    z-index: 20;
  }

  .chat-messages {
    max-height: 150px;
    overflow-y: auto;
    padding: 8px;
    background: rgba(0, 0, 0, 0.4);
    border-radius: 8px 8px 0 0;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .chat-msg {
    font-size: 12px;
    font-family: Georgia, serif;
    color: rgba(255, 255, 255, 0.8);
    word-wrap: break-word;
  }

  .chat-sender {
    color: #ffe680;
    font-weight: bold;
    margin-right: 4px;
  }

  :global(.chat-mention) {
    color: #67e8f9;
    font-weight: bold;
  }

  .chat-input-row {
    display: flex;
  }

  .chat-input {
    flex: 1;
    background: rgba(255, 255, 255, 0.08);
    border: 1px solid rgba(255, 255, 255, 0.15);
    border-radius: 0 0 0 8px;
    padding: 8px 12px;
    font-size: 13px;
    color: #fff;
    font-family: Georgia, serif;
    outline: none;
  }

  .chat-input:focus {
    border-color: rgba(255, 220, 80, 0.5);
  }

  .chat-input::placeholder {
    color: rgba(255, 255, 255, 0.3);
  }

  .chat-send {
    background: rgba(255, 220, 80, 0.15);
    border: 1px solid rgba(255, 220, 80, 0.3);
    border-radius: 0 0 8px 0;
    padding: 8px 16px;
    color: #ffe680;
    font-family: Georgia, serif;
    cursor: pointer;
    font-size: 13px;
  }

  .chat-send:hover {
    background: rgba(255, 220, 80, 0.25);
  }

  .toast {
    position: fixed;
    left: 50%;
    transform: translateX(-50%);
    background: rgba(255, 220, 80, 0.2);
    border: 1px solid rgba(255, 220, 80, 0.4);
    border-radius: 8px;
    padding: 12px 24px;
    color: #ffe680;
    font-family: Georgia, serif;
    font-size: 14px;
    z-index: 30;
    pointer-events: none;
    animation: toastFade 4s forwards;
    max-width: 400px;
    text-align: center;
  }

  @keyframes toastFade {
    0% { opacity: 0; transform: translateX(-50%) translateY(-10px); }
    10% { opacity: 1; transform: translateX(-50%) translateY(0); }
    80% { opacity: 1; transform: translateX(-50%) translateY(0); }
    100% { opacity: 0; transform: translateX(-50%) translateY(0); }
  }

  @media (max-width: 500px) {
    .game-title {
      font-size: 2rem;
    }
    .login-form input {
      width: 180px;
    }
  }
</style>
