package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	pb "github.com/binhbb2204/Manga-Hub-Group13/proto/manga"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// detectGRPCServer tries to find the gRPC server by querying the health endpoint
func detectGRPCServer() string {
	// Try localhost first
	targets := []string{"127.0.0.1:8080", "localhost:8080"}

	for _, target := range targets {
		resp, err := http.Get("http://" + target + "/health")
		if err == nil && resp.StatusCode == 200 {
			defer resp.Body.Close()
			var health map[string]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&health); err == nil {
				if localIP, ok := health["local_ip"].(string); ok {
					log.Printf("Detected gRPC server at %s:9092", localIP)
					return localIP + ":9092"
				}
			}
		}
	}

	log.Println("Could not detect server, falling back to localhost:9092")
	return "localhost:9092"
}

func main() {
	grpcTarget := detectGRPCServer()
	conn, err := grpc.NewClient(grpcTarget, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()
	c := pb.NewMangaServiceClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	log.Println("Testing GetManga...")
	r, err := c.GetManga(ctx, &pb.GetMangaRequest{Id: "one-piece"})
	if err != nil {
		log.Printf("could not get manga: %v", err)
	} else {
		log.Printf("Manga: %s, Author: %s", r.GetTitle(), r.GetAuthor())
	}

	log.Println("Testing SearchManga...")
	s, err := c.SearchManga(ctx, &pb.SearchRequest{Query: "One"})
	if err != nil {
		log.Printf("could not search manga: %v", err)
	} else {
		log.Printf("Found %d mangas", s.GetTotalCount())
		for _, m := range s.GetMangas() {
			log.Printf(" - %s", m.GetTitle())
		}
	}
}
