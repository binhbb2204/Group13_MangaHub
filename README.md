# Manga-Hub-Group13

## Project structure (Phase 1)

```
├── cmd/
│   ├── api-server/main.go
│   ├── tcp-server/main.go
│   ├── udp-server/main.go
│   └── grpc-server/main.go
├── internal/
│   ├── auth/
│   ├── manga/
│   ├── user/
│   ├── tcp/
│   ├── udp/
│   ├── websocket/
│   └── grpc/
├── pkg/
│   ├── models/
│   ├── database/
│   └── utils/
├── proto/
├── data/
├── docs/
├── docker-compose.yml
└── README.md
```

For Phase 1, only run the HTTP API server (`cmd/api-server`).

## Run the API server

```powershell
go build -o bin\api-server .\cmd\api-server
./bin/api-server.exe
```

Health check: http://localhost:8080/health

## Auth API examples (PowerShell)

### Register

```powershell
$body = @{ username = "johndoe"; email = "john@example.com"; password = "StrongP4ss" } | ConvertTo-Json
Invoke-RestMethod -Uri http://localhost:8080/auth/register -Method Post -ContentType 'application/json' -Body $body
```

Expected success includes: user_id, username, email, token, created_at, expires_at

Error examples:
- 409 Username already exists
- 400 Invalid email format
- 400 Password too weak (min 8 chars, mixed case, numbers)

### Login (username or email)

```powershell
$body = @{ username = "johndoe"; password = "StrongP4ss" } | ConvertTo-Json
$resp = Invoke-RestMethod -Uri http://localhost:8080/auth/login -Method Post -ContentType 'application/json' -Body $body
$token = $resp.token
```

Errors: 401 Invalid credentials; 401 Account not found

### Change password

```powershell
$body = @{ current_password = "StrongP4ss"; new_password = "N3wStrongP4ss" } | ConvertTo-Json
Invoke-RestMethod -Uri http://localhost:8080/auth/change-password -Method Post -ContentType 'application/json' -Headers @{ Authorization = "Bearer $token" } -Body $body
```

### Search manga

```powershell
Invoke-RestMethod -Uri "http://localhost:8080/manga?title=berserk&status=ongoing" -Method Get
```

## Configuration: Manga sources and API keys

This project can search manga using different external sources. You select the source with the `MANGA_SOURCE` environment variable and, where applicable, provide credentials via env vars.

- `MANGA_SOURCE` can be:
	- `jikan` (default): uses Jikan, no key required
	- `mangadex`: uses MangaDex official API; public search needs no key. To access authenticated endpoints or higher limits, set `MANGADEX_TOKEN`.
	- `rapidapi`: uses a RapidAPI-backed provider; requires API key and host

### MangaDex token (optional)

Public endpoints like search do not require an API key. If you need to call authenticated MangaDex endpoints, set an OAuth access token via:

- `MANGADEX_TOKEN` — The MangaDex access token (used as `Authorization: Bearer <token>`)

How to get a token (summary): Authenticate with MangaDex and obtain an access token (see MangaDex API docs). Once you have it, export it as an environment variable.

### RapidAPI configuration (alternative)

If you’re using a MangaDex wrapper on RapidAPI (or any other provider on RapidAPI), set:

- `RAPIDAPI_URL` — Base URL from the RapidAPI provider
- `RAPIDAPI_HOST` — The provider host (e.g., `example.p.rapidapi.com`)
- `RAPIDAPI_KEY` — Your RapidAPI key

Then set `MANGA_SOURCE=rapidapi`.

### Setting environment variables (Windows PowerShell)

```powershell
$env:MANGA_SOURCE = "mangadex"
$env:MANGADEX_TOKEN = "<your_mangadex_access_token>"  # optional
# or for RapidAPI
$env:MANGA_SOURCE = "rapidapi"
$env:RAPIDAPI_URL = "https://your-provider.p.rapidapi.com"
$env:RAPIDAPI_HOST = "your-provider.p.rapidapi.com"
$env:RAPIDAPI_KEY = "<your_rapidapi_key>"
```

## Notes

- By default, `MANGA_SOURCE` falls back to `jikan` if unset.
- With `MANGA_SOURCE=mangadex`, the app will read `MANGADEX_TOKEN` if present and send it as an `Authorization: Bearer` header. It’s optional for public search.
- With `MANGA_SOURCE=rapidapi`, all `RAPIDAPI_*` variables are required.