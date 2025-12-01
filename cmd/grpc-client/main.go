package main

import (
	"context"
	"log"
	"time"

	pb "github.com/binhbb2204/Manga-Hub-Group13/proto/manga"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	conn, err := grpc.NewClient("localhost:9092", grpc.WithTransportCredentials(insecure.NewCredentials()))
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
