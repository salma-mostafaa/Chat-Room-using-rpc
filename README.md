# Go RPC Chat

A concurrent terminal chat app in Go using `net/rpc`. One server and many clients, each running as a separate process.

## Communication approach: Polling

Every 500 ms, each client calls `ChatServer.GetMessages` to fetch its pending messages. The server keeps an **inbox** for each user, and sending a message just adds it to the other users' inboxes.

## Run

```bash
go run ./server        # terminal 1
go run ./client        # terminal 2, 3, 4 ... (one per user)
```

Client commands: `/users` lists online users, `/quit` leaves. Ctrl+C also leaves cleanly.

## RPC methods

| Method | Args | Reply |
|---|---|---|
| `ChatServer.Join` | username | bool |
| `ChatServer.Send` | `SendArgs{From, Text}` | bool |
| `ChatServer.GetMessages` | username | `[]Message` |
| `ChatServer.ListUsers` | username | `[]string` |
| `ChatServer.Leave` | username | bool |

## How the requirements are handled

**Duplicate usernames.** `Join` returns an error and the client asks again.

**Join/leave notifications.** These are sent as messages from `"SERVER"` to the other users.

**Broadcast excluding the sender.** `broadcast(from, text, except)` skips the sender's inbox.

**Concurrency.**
- `rpc.Accept` serves each client in its own goroutine.
- The users map is protected by a `sync.Mutex`.
- The client receives in one goroutine (polling) and reads input in `main`.

**Unexpected disconnection.** A background goroutine removes any user that hasn't polled for 5 seconds, and the others see "disconnected unexpectedly". If the server dies, the client prints "Disconnected from the server" and exits.

**Clean shutdown.**
- Client: Ctrl+C or `/quit` calls `Leave` (`sync.Once` makes sure it happens only once).
- Server: Ctrl+C sends "server is shutting down" to everyone, waits 1 second so clients receive it, then exits.
