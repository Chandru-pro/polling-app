# LivePoll

A live polling tool: create a poll, share the link, watch votes land in
real time with no refresh. Built for the GUVI × HCL developer internship task.

## Stack (all four doing real work)

| Layer    | Tech             | What it actually does |
|----------|------------------|------------------------|
| Frontend | React (Vite)     | Auth flows, poll creation, voting UI, live results |
| Backend  | Go (Gin)         | REST API, JWT auth, input validation, websocket upgrade |
| Database | MongoDB          | Users, polls, and a durable per-vote audit log (also the cold-start fallback for counts) |
| Realtime | Redis            | The live vote counters (`HINCRBY`) and pub/sub fan-out that drives every connected browser's chart, plus the atomic per-poll voter dedup (`SADD`) |

Redis is not decorative here: every vote hits Redis first (atomic dedup +
counter increment), and the resulting count is what gets published to the
poll's channel and broadcast over websockets. Mongo is the durable record —
if Redis ever comes up cold, results are rebuilt from the Mongo vote log
and Redis is reseeded.

## How the "live" part actually works

1. A voter casts a vote → `POST /api/polls/:id/vote`.
2. The backend validates the poll/option server-side, then does
   `SADD poll:{id}:voters {voterId}` — if that returns 0, they already
   voted, reject. If it returns 1, `HINCRBY poll:{id}:counts {optionIdx} 1`.
3. The vote is also written to MongoDB (`votes` collection) as a durable
   audit trail, with a unique index on `(pollId, voterId)` as a second line
   of defense against double voting.
4. The new tallies are published to a Redis pub/sub channel
   (`poll:{id}:updates`).
5. Every browser watching that poll's results page holds an open
   WebSocket (`GET /api/polls/:id/watch`); the backend has exactly one
   Redis subscriber per poll relaying messages to all of that poll's
   connected sockets. No polling, no refresh.

## Auth

Simple JWT-based signup/login, as the brief asked for — nothing elaborate.
Passwords are bcrypt-hashed. Creating and closing polls requires a valid
token; voting stays open to anyone with the link, since the audience
shouldn't need an account.

## Running locally

**Fastest path — Docker:**
```bash
docker compose up --build
```
Frontend: http://localhost:5173 · Backend: http://localhost:8080

**Manual path:**
```bash
# Terminal 1 — Mongo + Redis (or run them locally however you like)
docker run -d -p 27017:27017 mongo:7
docker run -d -p 6379:6379 redis:7-alpine

# Terminal 2 — backend
cd backend
cp .env.example .env
go mod tidy
go run main.go

# Terminal 3 — frontend
cd frontend
cp .env.example .env
npm install
npm run dev
```

## Deploying it for real

The brief requires an actually-reachable live link, not `localhost`. A
solid free/cheap path:

- **MongoDB**: MongoDB Atlas free tier (M0 cluster) → gives you a `mongodb+srv://…` URI.
- **Redis**: Upstash or Redis Cloud free tier → gives you a host:port + password.
- **Backend**: Render or Railway — point it at `/backend`, it builds the
  Dockerfile, set `MONGO_URI`, `REDIS_ADDR`, `REDIS_PASSWORD`, `JWT_SECRET`,
  `ALLOW_ORIGIN` (your deployed frontend URL) as environment variables.
- **Frontend**: Vercel or Netlify, or Render's static site — set
  `VITE_API_URL` to your deployed backend's URL at build time. If your
  backend is on plain `http://`, use `ws://` for the socket automatically
  (the code derives it from `VITE_API_URL`); if it's `https://`, it becomes
  `wss://` — make sure your platform actually proxies websocket upgrades
  (Render and Railway do; some static-only platforms don't for the backend
  side, which is why the backend needs a "real service" host, not a static
  one).

## Project structure

```
/backend    → Go (Gin) service: auth, poll CRUD, voting, websocket hub
/frontend   → React app: pages for auth, poll creation, voting, live results
docker-compose.yml → spins up mongo + redis + backend + frontend together
```

## Things I'd call out as deliberate scope decisions, not oversights

- One vote per browser is enforced via a client-generated `voterId` stored
  in `localStorage`, not per-account voting — the brief describes an open
  "audience," and real per-person identity would need real user accounts
  for voters too, which the brief explicitly doesn't ask for.
- Redis counters currently have no TTL/eviction policy — fine for a poll's
  natural lifetime in a demo/internship context; a production version
  would want an expiry + Mongo reconciliation job.
- No rate limiting on the vote endpoint yet — worth adding (e.g. per-IP)
  if this went further than the demo.

---

**A note if you used AI tools to help build this:** that's expected and
fine — just make sure you actually understand what's happening in each
piece (the Redis dedup/increment logic, the JWT flow, the websocket
relay), because the technical interview rounds will ask you to explain it
in depth, not just show that it runs. And don't forget: the walkthrough
video is a mandatory part of the submission, not optional.
