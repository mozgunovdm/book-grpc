package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"

	"github.com/joho/godotenv"
	booknpc "github.com/mozgunovdm/book-grpc/book"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
)

const defaultPort = "4401"

var collection *mongo.Collection

type server struct {
	booknpc.BookServiceServer
}

type bookItem struct {
	ID          primitive.ObjectID `bson:"_id,omitempty"`
	Code        string             `bson:"code"`
	Name        string             `bson:"name"`
	Author      string             `bson:"author"`
	Publisher   string             `bson:"publisher"`
	PublishedIn string             `bson:"published_in"`
}

func getBookData(data *bookItem) *booknpc.Book {
	return &booknpc.Book{
		Id:          data.ID.Hex(),
		Code:        data.Code,
		Name:        data.Name,
		Author:      data.Author,
		Publisher:   data.Publisher,
		PublishedIn: data.PublishedIn,
	}
}

func (*server) CreateBook(ctx context.Context, req *booknpc.CreateBookRequest) (*booknpc.CreateBookResponse, error) {
	log.Println("Create Book")
	book := req.GetBook()

	data := bookItem{
		Code:        book.GetCode(),
		Name:        book.GetName(),
		Author:      book.GetAuthor(),
		Publisher:   book.GetPublisher(),
		PublishedIn: book.GetPublishedIn(),
	}

	res, err := collection.InsertOne(ctx, data)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			fmt.Sprintf("Internal error: %v", err),
		)
	}
	oid, ok := res.InsertedID.(primitive.ObjectID)
	if !ok {
		return nil, status.Errorf(
			codes.Internal,
			fmt.Sprintf("Cannot convert to OID"),
		)
	}

	return &booknpc.CreateBookResponse{
		Book: &booknpc.Book{
			Id:          oid.Hex(),
			Code:        book.GetCode(),
			Name:        book.GetName(),
			Author:      book.GetAuthor(),
			Publisher:   book.GetPublisher(),
			PublishedIn: book.GetPublishedIn(),
		},
	}, nil
}

func (*server) ReadBook(ctx context.Context, req *booknpc.ReadBookRequest) (*booknpc.ReadBookResponse, error) {
	log.Println("Read Book")
	bookID := req.GetId()
	oid, err := primitive.ObjectIDFromHex(bookID)
	if err != nil {
		return nil, status.Errorf(
			codes.InvalidArgument,
			fmt.Sprintf("Cannot parse book code"),
		)
	}

	data := &bookItem{}
	filter := bson.M{"_id": oid}

	res := collection.FindOne(ctx, filter)
	if err := res.Decode(data); err != nil {
		return nil, status.Errorf(
			codes.NotFound,
			fmt.Sprintf("Cannot find book with specified ID: %v", err),
		)
	}

	return &booknpc.ReadBookResponse{
		Book: getBookData(data),
	}, nil
}

func (*server) UpdateBook(ctx context.Context, req *booknpc.UpdateBookRequest) (*booknpc.UpdateBookResponse, error) {
	log.Println("Updating Book")
	book := req.GetBook()
	oid, err := primitive.ObjectIDFromHex(book.GetId())
	log.Printf("Updating Book id %v", oid)
	if err != nil {
		return nil, status.Errorf(
			codes.InvalidArgument,
			fmt.Sprintf("Cannot parse ID"),
		)
	}

	data := &bookItem{}
	filter := bson.M{"_id": oid}

	res := collection.FindOne(ctx, filter)
	if err := res.Decode(data); err != nil {
		return nil, status.Errorf(
			codes.NotFound,
			fmt.Sprintf("Cannot find book with specified ID: %v", err),
		)
	}

	data.Code = book.GetCode()
	data.Name = book.GetName()
	data.Author = book.GetAuthor()
	data.Publisher = book.GetPublisher()
	data.PublishedIn = book.GetPublishedIn()

	_, updateErr := collection.ReplaceOne(context.Background(), filter, data)
	if updateErr != nil {
		return nil, status.Errorf(
			codes.Internal,
			fmt.Sprintf("Cannot update object in MongoDB: %v", updateErr),
		)
	}

	return &booknpc.UpdateBookResponse{
		Book: getBookData(data),
	}, nil

}

func (*server) DeleteBook(ctx context.Context, req *booknpc.DeleteBookRequest) (*booknpc.DeleteBookResponse, error) {
	log.Println("Deleting Book")
	oid, err := primitive.ObjectIDFromHex(req.GetCode())
	if err != nil {
		return nil, status.Errorf(
			codes.InvalidArgument,
			fmt.Sprintf("Cannot parse id"),
		)
	}

	filter := bson.M{"_id": oid}

	res, err := collection.DeleteOne(ctx, filter)

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			fmt.Sprintf("Cannot delete object in MongoDB: %v", err),
		)
	}

	if res.DeletedCount == 0 {
		return nil, status.Errorf(
			codes.NotFound,
			fmt.Sprintf("Cannot find book in MongoDB: %v", err),
		)
	}

	return &booknpc.DeleteBookResponse{Code: req.GetCode()}, nil
}

func (*server) ListBook(_ *booknpc.ListBookRequest, stream booknpc.BookService_ListBookServer) error {
	log.Println("List Book")
	cur, err := collection.Find(context.Background(), primitive.D{{}})
	if err != nil {
		return status.Errorf(
			codes.Internal,
			fmt.Sprintf("Unknown internal error: %v", err),
		)
	}
	defer cur.Close(context.Background())
	for cur.Next(context.Background()) {
		data := &bookItem{}
		err := cur.Decode(data)
		if err != nil {
			return status.Errorf(
				codes.Internal,
				fmt.Sprintf("Error while decoding data from MongoDB: %v", err),
			)

		}
		stream.Send(&booknpc.ListBookResponse{Book: getBookData(data)})
	}
	if err := cur.Err(); err != nil {
		return status.Errorf(
			codes.Internal,
			fmt.Sprintf("Unknown internal error: %v", err),
		)
	}
	return nil
}

func main() {

	err := godotenv.Load(".env")
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}

	mongo_url := os.Getenv("MONGODB_URL")

	// get the file name and line number
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	log.Println("Connecting to MongoDB")

	client, err := mongo.NewClient(options.Client().ApplyURI(mongo_url))
	if err != nil {
		log.Fatal(err)
	}
	err = client.Connect(context.TODO())
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		log.Println("Closing MongoDB Connection")
		if err := client.Disconnect(context.TODO()); err != nil {
			log.Fatalf("Error on disconnection with MongoDB : %v", err)
		}
	}()

	log.Println("Book Service Started")
	collection = client.Database("bookdb").Collection("book")

	lis, err := net.Listen("tcp", "0.0.0.0:"+port)
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	var opts []grpc.ServerOption
	srv := grpc.NewServer(opts...)
	booknpc.RegisterBookServiceServer(srv, &server{})
	reflection.Register(srv)

	go func() {
		log.Println("Starting Server...")
		if err := srv.Serve(lis); err != nil {
			log.Fatalf("failed to serve: %v", err)
		}
	}()

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)
	<-ch

	log.Println("Stopping the server")
	srv.Stop()
	log.Println("End of Program")
}
