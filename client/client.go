package main

import (
	"bufio"
	"fmt"
	"log"
	"net/rpc"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"

	"chat/commons"
)

var (
	client   *rpc.Client
	username string
	exitOnce sync.Once // Ctrl+C and /quit could both try to exit
)

func main() {
	var err error
	client, err = rpc.Dial("tcp", "127.0.0.1:42586")
	if err != nil {
		log.Fatal("Cannot connect to server: ", err)
	}

	in := bufio.NewReader(os.Stdin)

	// 1. Join with a username (ask again if it's taken)
	for {
		fmt.Print("Enter your username: ")
		line, err := in.ReadString('\n')
		if err != nil {
			return
		}
		username = strings.TrimSpace(line)

		var ok bool
		err = client.Call("ChatServer.Join", username, &ok)
		if err == nil {
			break
		}
		fmt.Println("Error:", err)
	}

	fmt.Println("==================== Welcome", username, "====================")
	fmt.Println("Type a message and press Enter.  /users = online users,  /quit = leave")

	// 2. Ctrl+C -> leave 
	go func() {
		stop := make(chan os.Signal, 1)
		signal.Notify(stop, os.Interrupt)
		<-stop
		leave()
	}()

	// 3. Receive messages in the background (polling)
	go receiveMessages()

	// 4. Read what the user types
	for {
		line, err := in.ReadString('\n')
		if err != nil {
			leave()
		}
		text := strings.TrimSpace(line)

		switch text {
		case "":
			continue
		case "/quit":
			leave()
		case "/users":
			var users []string
			if err := client.Call("ChatServer.ListUsers", username, &users); err != nil {
				fmt.Println("Error:", err)
				continue
			}
			fmt.Printf("Online (%d): %s\n", len(users), strings.Join(users, ", "))
		default:
			var ok bool
			args := commons.SendArgs{From: username, Text: text}
			if err := client.Call("ChatServer.Send", args, &ok); err != nil {
				fmt.Println("Error:", err)
			}
		}
	}
}

// receiveMessages asks the server for new messages every 500 ms
func receiveMessages() {
	for {
		var messages []commons.Message
		err := client.Call("ChatServer.GetMessages", username, &messages)
		if err != nil {
			exitOnce.Do(func() {
				fmt.Println("\nDisconnected from the server.")
				os.Exit(1)
			})
		}

		for _, m := range messages {
			if m.From == "SERVER" {
				fmt.Printf("[%s] *** %s\n", m.Time, m.Text)
			} else {
				fmt.Printf("[%s] %s: %s\n", m.Time, m.From, m.Text)
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// leave tells the server we are leaving, then exits
func leave() {
	exitOnce.Do(func() {
		var ok bool
		client.Call("ChatServer.Leave", username, &ok)
		fmt.Println("\nYou left the chat. Bye!")
		os.Exit(0)
	})
}
