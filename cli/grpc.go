package cli

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	pb "github.com/binhbb2204/Manga-Hub-Group13/proto/manga"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	grpcMangaID     string
	grpcSearchQuery string
	grpcChapter     int32
)

var grpcCmd = &cobra.Command{
	Use:   "grpc",
	Short: "Interact with the gRPC service",
	Long:  `Commands to interact with the MangaHub gRPC service directly.`,
}

var grpcMangaCmd = &cobra.Command{
	Use:   "manga",
	Short: "Manga related gRPC commands",
}

var grpcMangaGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Get manga details via gRPC",
	Run: func(cmd *cobra.Command, args []string) {
		conn, client := getGrpcClient()
		defer conn.Close()

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		r, err := client.GetManga(ctx, &pb.GetMangaRequest{Id: grpcMangaID})
		if err != nil {
			log.Fatalf("could not get manga: %v", err)
		}

		fmt.Printf("ID: %s\nTitle: %s\nAuthor: %s\nStatus: %s\nChapters: %d\nGenres: %v\nDescription: %s\n",
			r.GetId(), r.GetTitle(), r.GetAuthor(), r.GetStatus(), r.GetTotalChapters(), r.GetGenres(), r.GetDescription())
	},
}

var grpcMangaSearchCmd = &cobra.Command{
	Use:   "search",
	Short: "Search manga via gRPC",
	Run: func(cmd *cobra.Command, args []string) {
		conn, client := getGrpcClient()
		defer conn.Close()

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		r, err := client.SearchManga(ctx, &pb.SearchRequest{Query: grpcSearchQuery})
		if err != nil {
			log.Fatalf("could not search manga: %v", err)
		}

		fmt.Printf("Found %d mangas:\n", r.GetTotalCount())
		for _, m := range r.GetMangas() {
			fmt.Printf("- %s (%s)\n", m.GetTitle(), m.GetId())
		}
	},
}

var grpcProgressCmd = &cobra.Command{
	Use:   "progress",
	Short: "Progress related gRPC commands",
}

var grpcProgressUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update reading progress via gRPC",
	Run: func(cmd *cobra.Command, args []string) {
		conn, client := getGrpcClient()
		defer conn.Close()

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		// For demo purposes, using a hardcoded user ID or could be added as flag
		userID := "user123"

		r, err := client.UpdateProgress(ctx, &pb.ProgressRequest{
			UserId:  userID,
			MangaId: grpcMangaID,
			Chapter: grpcChapter,
		})
		if err != nil {
			log.Fatalf("could not update progress: %v", err)
		}

		if r.GetSuccess() {
			fmt.Println("Progress updated successfully")
		} else {
			fmt.Printf("Failed to update progress: %s\n", r.GetMessage())
		}
	},
}

func init() {
	grpcMangaGetCmd.Flags().StringVar(&grpcMangaID, "id", "", "Manga ID")
	grpcMangaGetCmd.MarkFlagRequired("id")

	grpcMangaSearchCmd.Flags().StringVar(&grpcSearchQuery, "query", "", "Search query")
	grpcMangaSearchCmd.MarkFlagRequired("query")

	grpcProgressUpdateCmd.Flags().StringVar(&grpcMangaID, "manga-id", "", "Manga ID")
	grpcProgressUpdateCmd.MarkFlagRequired("manga-id")
	grpcProgressUpdateCmd.Flags().Int32Var(&grpcChapter, "chapter", 0, "Chapter number")
	grpcProgressUpdateCmd.MarkFlagRequired("chapter")

	grpcMangaCmd.AddCommand(grpcMangaGetCmd)
	grpcMangaCmd.AddCommand(grpcMangaSearchCmd)
	grpcProgressCmd.AddCommand(grpcProgressUpdateCmd)

	grpcCmd.AddCommand(grpcMangaCmd)
	grpcCmd.AddCommand(grpcProgressCmd)
}

func getGrpcClient() (*grpc.ClientConn, pb.MangaServiceClient) {
	port := os.Getenv("GRPC_PORT")
	if port == "" {
		port = ":9092"
	}
	// Ensure port starts with : if it's just a number, but usually env var is :PORT or PORT
	// If it's just a number, prepend :
	if len(port) > 0 && port[0] != ':' {
		port = ":" + port
	}

	// For localhost connection
	target := "localhost" + port

	conn, err := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}

	return conn, pb.NewMangaServiceClient(conn)
}
