package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/joho/godotenv"
	booknpc "github.com/mozgunovdm/book-grpc/book"
	"google.golang.org/grpc"
)

const defaultPort = "4401"

func main() {

	log.Println("Book Client")

	err := godotenv.Load(".env")
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}

	opts := grpc.WithInsecure()

	cc, err := grpc.Dial("localhost:"+port, opts)
	if err != nil {
		log.Fatalf("could not connect: %v", err)
	}
	defer cc.Close() // Maybe this should be in a separate function and the error handled?

	c := booknpc.NewBookServiceClient(cc)

	// create 1
	log.Println("Creating the book 1")
	book_1 := &booknpc.Book{
		Code:        "600—649",
		Name:        "Snowman",
		Author:      "John Read",
		Publisher:   "Vid",
		PublishedIn: "1968",
	}
	createRes_1, err := c.CreateBook(context.Background(), &booknpc.CreateBookRequest{Book: book_1})
	if err != nil {
		log.Fatalf("Unexpected error: %v", err)
	}
	log.Printf("Book has been created: %v", createRes_1)

	//create 2
	log.Println("Creating the book 2")
	book_2 := &booknpc.Book{
		Code:        "456—778",
		Name:        "House on fire",
		Author:      "Den Braun",
		Publisher:   "Prosto",
		PublishedIn: "1999",
	}
	createRes_2, err := c.CreateBook(context.Background(), &booknpc.CreateBookRequest{Book: book_2})
	if err != nil {
		log.Fatalf("Unexpected error: %v", err)
	}
	log.Printf("Book has been created: %v", createRes_2)

	bookID := createRes_2.GetBook().GetId()
	bookOld := createRes_2.GetBook()

	// read
	log.Println("Reading the book")
	readReq := &booknpc.ReadBookRequest{Id: bookID}
	readRes, readErr := c.ReadBook(context.Background(), readReq)
	if readErr != nil {
		log.Printf("Error happened while reading: %v \n", readErr)
	}

	log.Printf("Book was read: %v \n", readRes)

	// update
	changedBook := &booknpc.Book{
		Id:          bookID,
		Code:        bookOld.GetCode(),
		Name:        bookOld.GetName(),
		Author:      bookOld.GetAuthor(),
		Publisher:   bookOld.GetPublisher(),
		PublishedIn: "2001",
	}
	updateRes, updateErr := c.UpdateBook(context.Background(), &booknpc.UpdateBookRequest{Book: changedBook})
	if updateErr != nil {
		log.Printf("Error happened while updating: %v \n", updateErr)
	}
	log.Printf("Book was updated: %v\n", updateRes)

	// list
	stream, err := c.ListBook(context.Background(), &booknpc.ListBookRequest{})
	if err != nil {
		log.Fatalf("error while calling ListBook RPC: %v", err)
	}
	for {
		res, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalf("Something happened: %v", err)
		}
		fmt.Println(res.GetBook())
	}
}
