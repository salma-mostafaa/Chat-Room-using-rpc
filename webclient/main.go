// Web client: same RPC calls as the terminal client, but the user types in a
// browser window that looks like a terminal.
//
//	go run ./webclient              -> open http://localhost:8080
//	go run ./webclient -port 8081   -> a second user on another port
package main

import (
	_ "embed"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/rpc"
	"os"
	"os/signal"
	"sync"

	"chat/commons"
)

//go:embed index.html
var page []byte // index.html is built into the program

var (
	client   *rpc.Client
	mu       sync.Mutex // protects username (HTTP handlers run in parallel goroutines)
	username string
	exitOnce sync.Once
)

func main() {
	port := flag.Int("port", 8080, "port for the browser page")
	server := flag.String("server", "127.0.0.1:42586", "chat server address")
	flag.Parse()

	var err error
	client, err = rpc.Dial("tcp", *server)
	if err != nil {
		log.Fatal("Cannot connect to server: ", err)
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(page)
	})
	http.HandleFunc("/join", handleJoin)
	http.HandleFunc("/send", handleSend)
	http.HandleFunc("/messages", handleMessages)
	http.HandleFunc("/users", handleUsers)
	http.HandleFunc("/leave", handleLeave)

	// Ctrl+C in this terminal -> leave cleanly
	go func() {
		stop := make(chan os.Signal, 1)
		signal.Notify(stop, os.Interrupt)
		<-stop
		leave()
		exitOnce.Do(func() {
			fmt.Println("\nWeb client stopped.")
			os.Exit(0)
		})
	}()

	addr := fmt.Sprintf("127.0.0.1:%d", *port)
	fmt.Printf("Open http://localhost:%d in your browser (Ctrl+C to stop)\n", *port)
	log.Fatal(http.ListenAndServe(addr, nil))
}

// ---------- HTTP handlers: each one just makes one RPC call ----------

func handleJoin(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	defer mu.Unlock()
	if username != "" {
		reply(w, "already joined as "+username, nil)
		return
	}
	name := r.FormValue("name")
	var ok bool
	if err := client.Call("ChatServer.Join", name, &ok); err != nil {
		reply(w, err.Error(), nil)
		return
	}
	username = name
	fmt.Println("Joined as", name)
	reply(w, "", nil)
}

func handleSend(w http.ResponseWriter, r *http.Request) {
	args := commons.SendArgs{From: currentUser(), Text: r.FormValue("text")}
	var ok bool
	err := client.Call("ChatServer.Send", args, &ok)
	reply(w, errText(err), nil)
}

func handleMessages(w http.ResponseWriter, r *http.Request) {
	var messages []commons.Message
	err := client.Call("ChatServer.GetMessages", currentUser(), &messages)
	reply(w, errText(err), messages)
}

func handleUsers(w http.ResponseWriter, r *http.Request) {
	var users []string
	err := client.Call("ChatServer.ListUsers", currentUser(), &users)
	reply(w, errText(err), users)
}

func handleLeave(w http.ResponseWriter, r *http.Request) {
	leave()
	reply(w, "", nil)
}

// ---------- helpers ----------

func leave() {
	mu.Lock()
	defer mu.Unlock()
	if username == "" {
		return
	}
	var ok bool
	client.Call("ChatServer.Leave", username, &ok)
	fmt.Println(username, "left the chat")
	username = ""
}

func currentUser() string {
	mu.Lock()
	defer mu.Unlock()
	return username
}

func errText(err error) string {
	if err != nil {
		return err.Error()
	}
	return ""
}

// reply sends {"error": "...", "data": ...} back to the page.
func reply(w http.ResponseWriter, errMsg string, data any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"error": errMsg, "data": data})
}
