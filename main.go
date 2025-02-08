package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/freeKrpark/ncp-object-storage-terminal/client"
	"github.com/freeKrpark/ncp-object-storage-terminal/command"

	"golang.org/x/term"
)

type Terminal struct {
	history      []string
	historyIndex int
	maxHistory   int
}

var app command.Command

func main() {
	path, err := os.Getwd()
	if err != nil {
		log.Fatalf("failed to read path")
	}
	app.Path = path
	app.Client = client.NewObjectClient()
	intro()
	doneChan := make(chan bool)
	go readUserInput(os.Stdin, doneChan)

	<-doneChan
	close(doneChan)
	fmt.Println("GoodBye.  ")
}

func readUserInput(in io.Reader, doneChan chan bool) {
	scanner := bufio.NewScanner(in)
	for {
		res, done := query(scanner)
		if done {
			doneChan <- true
			return
		}
		fmt.Println(res)
		prompt()
	}
}

func query(scanner *bufio.Scanner) (string, bool) {
	if !scanner.Scan() {
		return "Scanner error", true
	}

	text := strings.TrimSpace(scanner.Text())

	handlers := map[string]func(string) (string, bool){
		"q":            app.HandleExit,
		"exit":         app.HandleExit,
		"ls":           app.HandleLS,
		"show buckets": app.HandleShowBuckets,
		"list bucket":  app.HandleListBucket,
		"count bucket": app.HandleCountBucket,
		"start upload": app.HandleStartUpload,
	}

	prefixHandlers := map[string]func(string) (string, bool){
		"cd ":         app.HandleCD,
		"use ":        app.HandleUseBucket,
		"set ":        app.HandleSetS3Dir,
		"workers ":    app.HandleSetWorkers,
		"breakpoint ": app.HandleSetBreakPoint,
	}

	if handler, exists := handlers[strings.ToLower(text)]; exists {
		return handler(text)
	}

	for prefix, handler := range prefixHandlers {
		if strings.HasPrefix(strings.ToLower(text), prefix) {
			return handler(text)
		}
	}

	return fmt.Sprintf("%s: command not found", text), false
}

func intro() {
	width := getTerminalWidth()

	fmt.Println("Welcome Object Storage Terminal")
	fmt.Println(strings.Repeat("-", width))
	fmt.Println("Enter q to quit.")

}

func prompt() {
	hostname, _ := os.Hostname()
	username := os.Getenv("USERNAME")
	fmt.Printf("%s@%s:%s$ ", username, hostname, app.Path)
}

func getTerminalWidth() int {
	if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil {
		return w
	}
	return 80
}
