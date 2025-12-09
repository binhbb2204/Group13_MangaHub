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
go build -o bin/mangahub.exe ./cmd/mangahub/main.go (to build only mangahub)
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

Or use email instead:
```bash
mangahub auth login --email your@email.com
```

### Manage Your Account

**Change your password:**
```bash
mangahub auth change-password

# It'll ask for your current password and new password
Current password: ********
New password: ********
Confirm new password: ********
```

**Update your email:**
```bash
mangahub auth update-email

# Interactive prompt for new email (token required)
New email: newemail@example.com
```

**Update your username:**
```bash
mangahub auth update-username

# Interactive prompt for new username (token required)
New username: newusername
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
- **Change password:** `POST http://localhost:8080/auth/change-password`
- **Update email:** `POST http://localhost:8080/auth/update-email`
- **Update username:** `POST http://localhost:8080/auth/update-username`

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
Or use email:
```
POST http://localhost:8080/auth/login
Content-Type: application/json

{
  "email": "your@email.com",
  "password": "yourpassword"
}
```
Save the `token` from the response - you'll need it for protected endpoints!

**Change password (needs token):**
```
POST http://localhost:8080/auth/change-password
Authorization: Bearer your-token-here
Content-Type: application/json

{
  "current_password": "youroldpassword",
  "new_password": "yournewpassword"
}
```
Response:
```json
{
  "message": "Password changed successfully"
}
```

**Update email (needs token):**
```
POST http://localhost:8080/auth/update-email
Authorization: Bearer your-token-here
Content-Type: application/json

{
  "new_email": "newemail@example.com"
}
```
Response:
```json
{
  "message": "Email updated successfully",
  "user_id": "user123",
  "email": "newemail@example.com",
  "updated_at": "2025-12-09T10:30:00Z"
}
```

**Update username (needs token):**
```
POST http://localhost:8080/auth/update-username
Authorization: Bearer your-token-here
Content-Type: application/json

{
  "new_username": "newusername"
}
```
Response:
```json
{
  "message": "Username updated successfully",
  "user_id": "user123",
  "username": "newusername",
  "updated_at": "2025-12-09T10:30:00Z"
}
```

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

## Complete Testing Guide

### 🔍 Search & Ranking API Tests

#### **Test Case 1: Search with No Limit (Fetch ALL Results)**
Fetches all available results from MAL (up to 500 max), returns as array of 20-item pages.

**CLI:**
```bash
mangahub manga search "naruto"
```

**API:**
```bash
# Returns array of paginated objects (up to 500 results total)
curl "http://localhost:8080/manga/search?q=naruto"
```

#### **Test Case 2: Search with Limit (Cap Total Results)**
Fetches only the specified number of results, returns as array of 20-item pages.

**CLI:**
```bash
mangahub manga search "naruto" --limit 50
```

**API:**
```bash
# Returns array with exactly 50 results split into 3 pages (20, 20, 10)
curl "http://localhost:8080/manga/search?q=naruto&limit=50"

# Returns array with exactly 100 results split into 5 pages (20 each)
curl "http://localhost:8080/manga/search?q=naruto&limit=100"
```

#### **Test Case 3: Search Specific Page**
Returns only the requested page (20 items) from all available results.

**CLI:**
```bash
mangahub manga search "naruto" --page 1
mangahub manga search "naruto" --page 2
```

**API:**
```bash
# Returns page 1 (20 items) from ALL results
curl "http://localhost:8080/manga/search?q=naruto&page=1"

# Returns page 2 (20 items) from ALL results
curl "http://localhost:8080/manga/search?q=naruto&page=2"
```

#### **Test Case 4: Search Specific Page with Limit**
Returns only the requested page from the limited result set.

**CLI:**
```bash
mangahub manga search "naruto" --limit 100 --page 1
mangahub manga search "naruto" --limit 100 --page 3
```

**API:**
```bash
# Returns page 1 (20 items) from first 100 results
curl "http://localhost:8080/manga/search?q=naruto&limit=100&page=1"

# Returns page 3 (20 items) from first 100 results
curl "http://localhost:8080/manga/search?q=naruto&limit=100&page=3"
```

#### **Test Case 5: Ranking with No Limit**
Fetches all available ranking results (up to 500), returns as array of 20-item pages.

**CLI:**
```bash
mangahub manga ranking all
mangahub manga ranking bypopularity
mangahub manga ranking favorite
```

**API:**
```bash
# Get ALL top-ranked manga (up to 500)
curl "http://localhost:8080/manga/ranking?type=all"

# Get ALL most popular manga
curl "http://localhost:8080/manga/ranking?type=bypopularity"

# Get ALL most favorited manga
curl "http://localhost:8080/manga/ranking?type=favorite"
```

#### **Test Case 6: Ranking with Limit**
Fetches only the specified number of ranking results.

**CLI:**
```bash
mangahub manga ranking bypopularity --limit 100
mangahub manga ranking all --limit 50
```

**API:**
```bash
# Get only top 100 by popularity (5 pages of 20 each)
curl "http://localhost:8080/manga/ranking?type=bypopularity&limit=100"

# Get only top 50 overall (3 pages: 20, 20, 10)
curl "http://localhost:8080/manga/ranking?type=all&limit=50"

# Get only top 25 favorites (2 pages: 20, 5)
curl "http://localhost:8080/manga/ranking?type=favorite&limit=25"
```

#### **Test Case 7: Ranking Specific Page**
Returns only the requested page from ranking results.

**CLI:**
```bash
mangahub manga ranking bypopularity --page 1
mangahub manga ranking all --page 5
```

**API:**
```bash
# Get page 1 of popular manga
curl "http://localhost:8080/manga/ranking?type=bypopularity&page=1"

# Get page 5 of all rankings
curl "http://localhost:8080/manga/ranking?type=all&page=5"
```

#### **Test Case 8: Featured Manga (Homepage)**
Returns curated lists for homepage display.

**API:**
```bash
# Get featured manga sections (Top Ranked, Most Popular, Most Favorited)
curl "http://localhost:8080/manga/featured"
```

#### **Test Case 9: Manga Details**
Get full information about a specific manga.

**CLI:**
```bash
mangahub manga info 13      # One Piece
mangahub manga info 21      # Death Note
mangahub manga info 2       # Berserk
```

**API:**
```bash
# Get detailed information
curl "http://localhost:8080/manga/info/13"
curl "http://localhost:8080/manga/info/21"
```

### 👤 Authentication Tests

#### **Test Case 10: User Registration**

**CLI:**
```bash
mangahub auth register --username testuser --email test@example.com
# Password will be prompted securely
```

**API:**
```bash
curl -X POST "http://localhost:8080/auth/register" \
  -H "Content-Type: application/json" \
  -d '{
    "username": "testuser",
    "email": "test@example.com",
    "password": "SecurePass123"
  }'
```

#### **Test Case 11: User Login**

**CLI:**
```bash
mangahub auth login --username testuser
# Password will be prompted securely
```

**API:**
```bash
curl -X POST "http://localhost:8080/auth/login" \
  -H "Content-Type: application/json" \
  -d '{
    "username": "testuser",
    "password": "SecurePass123"
  }'
# Save the returned token for authenticated requests
```

#### **Test Case 12: User Logout**

**CLI:**
```bash
mangahub auth logout
```

### 📚 Library Management Tests (Requires Authentication)

#### **Test Case 13: Add Manga to Library**

**CLI:**
```bash
mangahub library add --manga-id 13 --status reading
mangahub library add --manga-id 21 --status completed
mangahub library add --manga-id 2 --status plan_to_read
```

**API:**
```bash
curl -X POST "http://localhost:8080/users/library" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "manga_id": "13",
    "status": "reading"
  }'
```

Status options: `reading`, `completed`, `plan_to_read`, `on_hold`, `dropped`

#### **Test Case 14: View Library**

**CLI:**
```bash
mangahub library list
mangahub library list --status reading
mangahub library list --status completed
```

**API:**
```bash
# Get all library items
curl "http://localhost:8080/users/library" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN"

# Filter by status
curl "http://localhost:8080/users/library?status=reading" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN"
```

#### **Test Case 15: Update Reading Progress**

**CLI:**
```bash
mangahub progress update --manga-id 13 --chapter 1095
mangahub progress update --manga-id 21 --chapter 108
```

**API:**
```bash
curl -X PUT "http://localhost:8080/users/progress" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "manga_id": "13",
    "current_chapter": 1095
  }'
```

### 📊 Response Format Examples

#### Single Page Response
```json
{
  "mangas": [
    {
      "id": "2",
      "title": "Berserk",
      "author": "Kentarou Miura",
      "status": "currently_publishing",
      "total_chapters": 0,
      "cover_url": "https://cdn.myanimelist.net/images/manga/1/157897l.webp"
    }
    // ... more manga (up to 20)
  ],
  "pagination": {
    "page": 1,
    "limit": 20,
    "total": 500,
    "total_pages": 25,
    "has_next": true,
    "has_prev": false
  }
}
```

#### Multiple Pages Response (When no page specified)
```json
[
  {
    "mangas": [ /* 20 items */ ],
    "pagination": { "page": 1, "limit": 20, "total": 100, "total_pages": 5, "has_next": true, "has_prev": false }
  },
  {
    "mangas": [ /* 20 items */ ],
    "pagination": { "page": 2, "limit": 20, "total": 100, "total_pages": 5, "has_next": true, "has_prev": true }
  }
  // ... continues for all pages
]
```

### 🎯 Key Testing Points

**Pagination Behavior:**
- ✅ No `limit` → Fetches ALL results (up to 500)
- ✅ With `limit=N` → Fetches exactly N results (up to 500 max)
- ✅ Page size is fixed at 20 items per page
- ✅ `page` parameter → Returns only that specific page
- ✅ No `page` → Returns all pages as array

**CLI Defaults:**
- CLI defaults to `--page 1` for better performance
- Use `--limit` to cap total results
- Use `--page` to navigate through pages

**Expected Behavior:**
- `?q=naruto` → All results (500 max), 25 pages
- `?q=naruto&limit=100` → 100 results, 5 pages
- `?q=naruto&page=2` → Page 2 only (20 items) from all 500
- `?q=naruto&limit=100&page=3` → Page 3 only (20 items) from first 100
