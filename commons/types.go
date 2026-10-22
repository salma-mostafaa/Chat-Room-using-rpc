package commons

// Message is one chat message or notification.
// From == "server" means it is a notification (join / leave / shutdown).
type Message struct {
	From string //stores the senders name
	Text string //actual message
	Time string //time of the message
}

// SendArgs is what the client sends with Send.
type SendArgs struct {
	From string
	Text string
}
