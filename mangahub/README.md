# MangaHub

## Project Structure

See the directory tree above for the main structure. Each server (API, TCP, UDP, gRPC) has its own entry point in `cmd/`. Core logic is in `internal/`, shared code in `pkg/`, and protocol/data/docs in their respective folders.

## Phase 2 Setup

1. Install [Docker](https://www.docker.com/) and [Go](https://golang.org/).
2. Clone this repository.
3. Build and run all services:
   ```sh
   docker-compose up --build
   ```
4. Each service will be available on its respective port (see `docker-compose.yml`).

## Development
- Place your Go code in the appropriate folders as per the structure.
- Use `go mod tidy` to manage dependencies.
- Update this README as the project evolves.
