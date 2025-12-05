# MangaHub API - Postman Testing Guide

## 📦 Import the Collection

1. Open Postman
2. Click **Import** button
3. Select `MangaHub_API.postman_collection.json`
4. Collection will appear in your sidebar

## 🚀 Getting Started

### 1. Start the API Server

```powershell
go run cmd/api-server/main.go
```

The server will start on `http://localhost:8080`

### 2. Test Basic Endpoint

Send a request to `Health Check` to verify server is running.

## 🔑 Authentication Flow

### Register & Login

1. **Register** - Create a new account
   - Use the "Register" request
   - Body example:
     ```json
     {
       "username": "testuser",
       "email": "test@example.com",
       "password": "password123"
     }
     ```

2. **Login** - Get JWT token
   - Use the "Login" request
   - Token is automatically saved to collection variable
   - All protected endpoints will use this token

3. **Use Protected Endpoints**
   - Token is automatically included in requests
   - Try: Get Profile, Add to Library, etc.

## 📚 MangaDex Chapter Reading API

### New Endpoints for Reading Manga

#### 1. Get Chapters
**GET** `/manga/chapters/{mangadexId}?language=en&limit=100`

**Example:**
```
GET http://localhost:8080/manga/chapters/a1c7c817-4e59-43b7-9365-09675a149a6f?language=en&limit=100
```

**Parameters:**
- `mangadexId` (path): MangaDex manga UUID (e.g., from manga info)
- `language` (query): Chapter language (en, ja, es, etc.) - default: en
- `limit` (query): Max chapters to return (max 500) - default: 100

**Response:**
```json
{
  "mangadex_id": "a1c7c817-4e59-43b7-9365-09675a149a6f",
  "language": "en",
  "total": 50,
  "chapters": [
    {
      "id": "chapter-uuid",
      "chapter": "1",
      "title": "Romance Dawn",
      "pages": 53,
      "volume": "1",
      "language": "en",
      "readableAt": "2024-01-01T00:00:00+00:00"
    }
  ]
}
```

#### 2. Get Chapter Pages
**GET** `/manga/chapter/{chapterId}/pages`

**Example:**
```
GET http://localhost:8080/manga/chapter/e9fb431b-5a4d-4bf8-9c4a-0510c5eca6d0/pages
```

**Parameters:**
- `chapterId` (path): MangaDex chapter UUID (from chapters list)

**Response:**
```json
{
  "chapter_id": "e9fb431b-5a4d-4bf8-9c4a-0510c5eca6d0",
  "total_pages": 53,
  "base_url": "https://uploads.mangadex.org",
  "hash": "abc123...",
  "page_urls": [
    "https://uploads.mangadex.org/data/abc123.../page1.jpg",
    "https://uploads.mangadex.org/data/abc123.../page2.jpg"
  ]
}
```

## 🎯 Complete Reading Workflow

### Step-by-Step: From Search to Reading

1. **Search for Manga**
   ```
   GET /manga/search?q=One Piece
   ```
   Get MAL manga ID (e.g., `13`)

2. **Get Manga Details**
   ```
   GET /manga/info/13
   ```
   Response includes `mangadex_id` field

3. **List Chapters**
   ```
   GET /manga/chapters/{mangadex_id}?language=en&limit=50
   ```
   Get list of available chapters with their IDs

4. **Get Chapter Pages**
   ```
   GET /manga/chapter/{chapter_id}/pages
   ```
   Get direct URLs to all page images

5. **Display/Download Pages**
   - Use the URLs from `page_urls` array
   - Open in browser or download programmatically

## 📖 API Endpoints Reference

### Manga Discovery (MyAnimeList)
- `GET /manga/search` - Search manga
- `GET /manga/info/:id` - Get detailed info (includes MangaDex ID)
- `GET /manga/featured` - Get featured manga sections
- `GET /manga/ranking` - Get rankings

### MangaDex Reading
- `GET /manga/chapters/:mangadexId` - List chapters
- `GET /manga/chapter/:chapterId/pages` - Get page URLs

### User Library (Requires Auth)
- `GET /users/me` - Get profile
- `POST /users/library` - Add to library
- `GET /users/library` - Get library
- `PUT /users/progress` - Update progress
- `DELETE /users/library/:manga_id` - Remove from library

## 💡 Tips

### Variables
- `baseUrl`: Default is `http://localhost:8080`
- `token`: Automatically set after login

### Getting MangaDex IDs

**Method 1: From manga info**
```
GET /manga/info/13
```
Look for `mangadex_id` in response

**Method 2: MangaDex website**
- Go to https://mangadex.org/
- Search for manga
- Copy UUID from URL: `https://mangadex.org/title/{UUID}`

### Common MangaDex IDs for Testing
- One Piece: `a1c7c817-4e59-43b7-9365-09675a149a6f`
- Solo Leveling: `32d76d19-8a05-4db0-9fc2-e0b0648fe9d0`
- Naruto: `234e5e28-c0c8-4e66-b2c3-cf6466e92c1d`

## 🐛 Troubleshooting

### Server not responding
```powershell
# Check if server is running
curl http://localhost:8080/health
```

### 404 on chapter pages
Some chapters may not have scanlations available. Try different chapters from the list.

### Token expired
Re-run the Login request to get a new token.

## 🔗 Related Files
- Collection: `MangaHub_API.postman_collection.json`
- Server code: `cmd/api-server/main.go`
- Handlers: `internal/manga/handler.go`
- MangaDex API: `internal/manga/external.go`
