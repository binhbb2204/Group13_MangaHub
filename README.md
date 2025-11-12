# Manga-Hub-Group13
# WTF
A CLI-based manga management system that lets you search, track, and manage your manga collection using the MyAnimeList API.

## Getting Started

### Configuration
First things first! Create a `.env` file in your project root with these settings:

```env
# Server Ports (change these if you want different ports)
API_PORT=8080        # Your main API server
TCP_PORT=9090        # TCP server
UDP_PORT=9091        # UDP server
GRPC_PORT=9092       # gRPC server
WEBSOCKET_PORT=9093  # WebSocket server

# MyAnimeList API (get your client ID from https://myanimelist.net/apiconfig)
MAL_CLIENT_ID=your_actual_client_id_here

# Database & Auth
DB_PATH=./data/mangahub.db
JWT_SECRET=your-super-secret-jwt
FRONTEND_URL=http://localhost:3000
```

**Pro tip:** All ports are configurable, so if you're already using port 8080 for something else, just change `API_PORT` to whatever you like!

### Setting Everything Up

**Step 1: Start Your Server**

Open a terminal and fire up the API server:
```bash
go run cmd/api-server/main.go
```
You'll see it running on whatever port you set in `.env` (8080 by default). Keep this terminal open!

**Step 2: Build the CLI**

Open another terminal and build your CLI tool:
```bash
go build ./...    (to build all)
go build -o bin/mangahub.exe cmd/main.go (to build only mangahub)
```

**Step 3: Make It Accessible**

Add it to your PATH so you can use it from anywhere:
```powershell
$env:PATH = "D:\Net-Centric-Lab\Group13_MangaHub\bin;$env:PATH"
```

**Step 4: Initialize**

Now initialize the CLI configuration:
```bash
mangahub init
```
This creates a `.mangahub` folder in your project directory with all your settings and data.

## How to Use

### Your First Time? Create an Account!

```bash
# Sign up with your details
mangahub auth register --username yourname --email your@email.com

# It'll ask for your password (hidden for security)
Password: ********
Confirm Password: ********
```

Once you're registered, log in:
```bash
mangahub auth login --username yourname
```

When you're done, just:
```bash
mangahub auth logout
```

### Finding Manga

**Quick search** - Just type what you're looking for:
```bash
mangahub manga search "naruto"
```

**Filter by genre** - Want something specific?
```bash
mangahub manga search "romance" --genre Romance
```

**Filter by status** - Only want completed series?
```bash
mangahub manga search "action" --status completed
```

**Limit results** - Don't want to be overwhelmed?
```bash
mangahub manga search "one piece" --limit 5
```

**Get the full details** - Found something interesting? Get more info:
```bash
mangahub manga info 13  # That's the MAL ID you see in search results
```

```bash
mangahub manga featured

mangahub manga ranking all

mangahub manga ranking bypopularity

mangahub manga ranking favorite
```
### Managing Your Library

**Add a manga** - Found something you want to read?
```bash
mangahub library add --manga-id 13 --status reading
```

**Check your library** - See what you've collected:
```bash
mangahub library list
```

**Update your progress** - Just finished a chapter?
```bash
mangahub progress update --manga-id 13 --chapter 1095
```

## Testing with Postman for Frontend

Good news! The API is ready to go. Just remember the port you set in `.env` (default is 8080).

### Endpoints You Can Use Without Login:
- **Search manga:** `GET http://localhost:8080/manga/search?q=naruto`
- **Get manga details:** `GET http://localhost:8080/manga/info/:id`
- **Register:** `POST http://localhost:8080/auth/register`
- **Login:** `POST http://localhost:8080/auth/login`

### Need Authentication? (JWT Token Required):
- **Add to library:** `POST http://localhost:8080/users/library`
- **See your library:** `GET http://localhost:8080/users/library`
- **Update progress:** `PUT http://localhost:8080/users/progress`

**Quick tip:** After login, you'll get a JWT token. Add it to your request headers as `Authorization: Bearer <your-token>` for protected endpoints.

### Postman Examples

Want to test the same searches from the CLI? Here's how:

**Basic search (like `mangahub manga search "naruto" --limit 5`):**
```
GET http://localhost:8080/manga/search?q=naruto&limit=5
```

**Search with filters (like `mangahub manga search "romance" --genre romance --status completed`):**
```
GET http://localhost:8080/manga/search?q=romance
```
Note: The API returns results from MyAnimeList, and you'll need to filter by genre/status on the client side. The CLI does this automatically for you!

**Get manga details (like `mangahub manga info 13`):**
```
GET http://localhost:8080/manga/info/13
```

# Get featured manga for homepage
curl http://localhost:8080/manga/featured

# Get top ranked manga
curl http://localhost:8080/manga/ranking?type=all&limit=20

# Get most popular manga
curl http://localhost:8080/manga/ranking?type=bypopularity&limit=10

# Get most favorited manga
curl http://localhost:8080/manga/ranking?type=favorite&limit=15

**Register a new user:**
```
POST http://localhost:8080/auth/register
Content-Type: application/json

{
  "username": "yourname",
  "email": "your@email.com",
  "password": "yourpassword"
}
```

**Login:**
```
POST http://localhost:8080/auth/login
Content-Type: application/json

{
  "username": "yourname",
  "password": "yourpassword"
}
```
Save the `token` from the response - you'll need it for protected endpoints!

**Add to library (needs token):**
```
POST http://localhost:8080/users/library
Authorization: Bearer your-token-here
Content-Type: application/json

{
  "manga_id": 13,
  "status": "reading"
}
```

Want all the technical details? Check out `docs/API_ENDPOINTS.md` for the full API documentation with request/response examples.
