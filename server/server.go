package main

import (
	"errors"
	"fmt"
	"log"
	"net"
	"net/rpc"
	"os"
	"os/signal"
	"sort"
	"strings"
	"sync"
	"time"

	"chat/commons"
)

// User is everything the server keeps for one connected user.
type User struct {
	Inbox    []commons.Message // messages waiting until the user asks for them
	LastSeen time.Time         // last time the user asked for messages
}

// ChatServer is the RPC object. Many clients call it at the same time
// (each RPC call runs in its own goroutine), so the map is protected by a mutex.
type ChatServer struct {
	mu    sync.Mutex
	users map[string]*User
}

// broadcast puts a message in every inbox except the sender's ofcoursseee
// The caller must already hold the lock.
func (s *ChatServer) broadcast(from, text, except string) {
	msg := commons.Message{From: from, Text: text, Time: time.Now().Format("15:04:05")}
	for name, user := range s.users {
		if name != except {
			user.Inbox = append(user.Inbox, msg)
		}
	}
}

// removeUser deletes a user and tells everyone
func (s *ChatServer) removeUser(name, reason string) {
	delete(s.users, name)
	s.broadcast("SERVER", name+" "+reason, "")
	fmt.Printf("[-] %s %s (%d online)\n", name, reason, len(s.users))
}

// RPC Join
func (s *ChatServer) Join(name string, reply *bool) error {
	name = strings.TrimSpace(name)
	if name == "" || strings.Contains(name, " ") {
		return errors.New("username can't be empty or contain spaces")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.users[name]; exists {
		return errors.New("username already taken, choose another one")
	}
	s.users[name] = &User{LastSeen: time.Now()}
	s.broadcast("SERVER", name+" joined the chat", name)
	fmt.Printf("[+] %s joined (%d online)\n", name, len(s.users))
	*reply = true
	return nil
}

// RPC Send
func (s *ChatServer) Send(args commons.SendArgs, reply *bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.users[args.From]; !exists {
		return errors.New("you are not connected")
	}
	s.broadcast(args.From, args.Text, args.From) // everyone except the sender
	fmt.Printf("[msg] %s: %s\n", args.From, args.Text)
	*reply = true
	return nil
}

// RPC GetMessages (polling): returns and empties the user's inbox
func (s *ChatServer) GetMessages(name string, reply *[]commons.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	user, exists := s.users[name]
	if !exists {
		return errors.New("you are not connected")
	}
	user.LastSeen = time.Now()
	*reply = user.Inbox
	user.Inbox = nil
	return nil
}

// RPC ListUsers
func (s *ChatServer) ListUsers(name string, reply *[]string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for userName := range s.users {
		*reply = append(*reply, userName)
	}
	sort.Strings(*reply)
	return nil
}

// RPC Leave
func (s *ChatServer) Leave(name string, reply *bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.users[name]; exists {
		s.removeUser(name, "left the chat")
	}
	*reply = true
	return nil
}

// checkTimeouts runs in its own goroutine. A client polls every 500 ms,
// so if we hear nothing for 5 seconds the client crashed or lost its network.
func (s *ChatServer) checkTimeouts() {
	for {
		time.Sleep(1 * time.Second)

		s.mu.Lock()
		for name, user := range s.users {
			if time.Since(user.LastSeen) > 5*time.Second {
				s.removeUser(name, "disconnected unexpectedly")
			}
		}
		s.mu.Unlock()
	}
}

func main() {
	listener, err := net.Listen("tcp", "0.0.0.0:42586")
	if err != nil {
		log.Fatal(err)
	}

	server := &ChatServer{users: make(map[string]*User)}
	rpc.Register(server)

	go server.checkTimeouts()
	go rpc.Accept(listener) // serves every client in its own goroutine

	fmt.Println("Chat server running on port 42586 (Ctrl+C to stop)")

	// Wait for Ctrl+C
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	<-stop

	fmt.Println("\nShutting down...")
	server.mu.Lock()
	server.broadcast("SERVER", "server is shutting down", "")
	server.mu.Unlock()
	time.Sleep(1 * time.Second) // give clients time to receive the notice
	fmt.Println("Server stopped")
}
