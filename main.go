package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"
)

type Config struct {
	DatabaseURL string
	ServerPort  string
}

var db *pgx.Conn
var clients = make(map[string]Client)
var config Config
var mu sync.RWMutex

type Client struct {
	conn     net.Conn
	name     string
	pswdHash string
	curRoom  string
}

func loadConfig() Config {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:raw0@localhost:5432/tupochatdb"
		log.Println("WARNING: DATABASE_URL not set, using default:", dbURL)
	}

	port := os.Getenv("SERVER_PORT")
	if port == "" {
		port = "5522"
	}

	return Config{
		DatabaseURL: dbURL,
		ServerPort:  port,
	}
}

func connectDatabase() error {
	var err error
	db, err = pgx.Connect(context.Background(), config.DatabaseURL)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	log.Println("Connected to database")
	return nil
}

func closeDatabase() {
	db.Close(context.Background())
}

func newClient(name string, pswd string) error {
	_, err := db.Exec(context.Background(),
		"INSERT INTO clients (username, password_hash, current_room) VALUES ($1, $2, 'global')",
		name, pswd)
	if err != nil {
		return fmt.Errorf("failed to insert client: %w", err)
	}
	return nil
}

func getClient(name string) (Client, error) {
	var client Client
	err := db.QueryRow(context.Background(),
		"SELECT username, password_hash, current_room FROM clients WHERE username = $1",
		name).Scan(&client.name, &client.pswdHash, &client.curRoom)
	if err != nil {
		return client, err
	}
	return client, nil
}

func saveMessage(from string, message string, timestamp time.Time, room string) error {
	timeStr := timestamp.Format("2006-01-02 15:04:05")
	_, err := db.Exec(context.Background(),
		"INSERT INTO messages (sender, content, sent_at, room) VALUES ($1, $2, $3, $4)",
		from, message, timeStr, room)
	if err != nil {
		return fmt.Errorf("failed to insert message: %w", err)
	}
	return nil
}

func checkRoomOwner(name string) (string, error) {
	var owner string
	err := db.QueryRow(context.Background(),
		"SELECT owner FROM rooms WHERE name = $1", name).Scan(&owner)
	return owner, err
}

func newRoom(name string, owner string) (string, error) {
	var exists bool
	err := db.QueryRow(context.Background(),
		"SELECT EXISTS(SELECT 1 FROM rooms WHERE name = $1)", name).Scan(&exists)
	if err != nil {
		return "", fmt.Errorf("failed to check room existence: %w", err)
	}
	if exists {
		return "room " + name + " already exists\n", nil
	}

	_, err = db.Exec(context.Background(),
		"INSERT INTO rooms (name, owner) VALUES ($1, $2)", name, owner)
	if err != nil {
		return "", fmt.Errorf("failed to create room: %w", err)
	}

	_, err = db.Exec(context.Background(),
		"UPDATE clients SET current_room = $1 WHERE username = $2", name, owner)
	if err != nil {
		return "", fmt.Errorf("failed to join room: %w", err)
	}

	return "created room " + name + "\n", nil
}

func getHistory(client Client, room string) error {
	rows, err := db.Query(context.Background(),
		"SELECT content, sent_at, sender FROM messages WHERE room = $1 ORDER BY sent_at ASC LIMIT 500",
		room)
	if err != nil {
		return fmt.Errorf("failed to get history: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var msg, name, ts string
		if err := rows.Scan(&msg, &ts, &name); err != nil {
			log.Printf("Failed to scan row: %v", err)
			continue
		}
		client.conn.Write([]byte(ts + " " + name + ": " + msg + "\n"))
	}

	return rows.Err()
}

func listRooms() ([]string, error) {
	rows, err := db.Query(context.Background(), "SELECT name FROM rooms")
	if err != nil {
		return nil, fmt.Errorf("failed to list rooms: %w", err)
	}
	defer rows.Close()

	var rooms []string
	for rows.Next() {
		var room string
		if err := rows.Scan(&room); err != nil {
			continue
		}
		rooms = append(rooms, room)
	}
	return rooms, rows.Err()
}

func roomExists(name string) (bool, error) {
	var exists bool
	err := db.QueryRow(context.Background(),
		"SELECT EXISTS(SELECT 1 FROM rooms WHERE name = $1)", name).Scan(&exists)
	return exists, err
}

func joinRoom(name string, room string) error {
	_, err := db.Exec(context.Background(),
		"UPDATE clients SET current_room = $1 WHERE username = $2", room, name)
	return err
}

func deleteRoom(name string) error {
	_, err := db.Exec(context.Background(),
		"UPDATE clients SET current_room = 'global' WHERE current_room = $1", name)
	if err != nil {
		return err
	}
	_, err = db.Exec(context.Background(), "DELETE FROM rooms WHERE name = $1", name)
	return err
}

func parseMessage(msg string) (string, string) {
	split := strings.Split(msg, " ")

	if len(split) == 1 {
		return "", split[0]
	}
	return split[0], strings.Join(split[1:], " ")
}

func distribute(from string, message string, room string) {
	time := time.Now().Format("2006-01-02 15:04:05")
	msg := fmt.Sprintf("%s -- %s -- %s: %s\n", room, time, from, message)

	mu.RLock()
	for _, client := range clients {
		if client.curRoom == room {
			client.conn.Write([]byte(msg))
		}
	}
	mu.RUnlock()
}

func handleConnection(c Client) {
	defer c.conn.Close()
	reader := bufio.NewReader(c.conn)

	for {
		data, err := reader.ReadString('\n')
		if err != nil {
			fmt.Printf("%s disconnected\n", c.name)
			return
		}

		cmd, msg := parseMessage(strings.TrimSuffix(data, "\n"))

		switch cmd {
		case "/exit":
			fmt.Printf("client %s exit chat\n", c.name)
			return

		case "/help":
			c.conn.Write([]byte("commands:\n/room - create and join room\n/join - join room\n/deleteRoom - delete room\n/list - list rooms\n/exit - exit chat\n"))

		case "/list":
			rooms, err := listRooms()
			if err != nil {
				c.conn.Write([]byte("Error listing rooms\n"))
				log.Printf("Error listing rooms: %v", err)
				continue
			}
			if len(rooms) == 0 {
				c.conn.Write([]byte("No rooms available\n"))
			}
			for _, room := range rooms {
				c.conn.Write([]byte(room + "\n"))
			}

		case "/room":
			if msg == "" || len(msg) > 20 {
				c.conn.Write([]byte("Room name must be 1-20 characters\n"))
				continue
			}
			exists, _ := roomExists(msg)
			if exists {
				c.conn.Write([]byte("Room " + msg + " already exists\n"))
				continue
			}
			result, err := newRoom(msg, c.name)
			if err != nil {
				c.conn.Write([]byte("Error creating room\n"))
				log.Printf("Error creating room: %v", err)
				continue
			}
			c.curRoom = msg
			mu.Lock()
			clients[c.name] = c
			mu.Unlock()
			c.conn.Write([]byte(result))

		case "/join":
			if msg == "" || len(msg) > 20 {
				c.conn.Write([]byte("Room name must be 1-20 characters\n"))
				continue
			}
			exists, _ := roomExists(msg)
			if !exists {
				c.conn.Write([]byte("Room " + msg + " does not exist\n"))
				continue
			}
			if err := joinRoom(c.name, msg); err != nil {
				c.conn.Write([]byte("Error joining room\n"))
				continue
			}
			c.curRoom = msg
			mu.Lock()
			clients[c.name] = c
			mu.Unlock()
			getHistory(c, c.curRoom)
			c.conn.Write([]byte("Joined " + msg + "\n"))

		case "/deleteRoom":
			if msg == "" || len(msg) > 20 {
				c.conn.Write([]byte("Room name must be 1-20 characters\n"))
				continue
			}
			exists, _ := roomExists(msg)
			if !exists {
				c.conn.Write([]byte("Room " + msg + " does not exist\n"))
				continue
			}
			owner, err := checkRoomOwner(msg)
			if err != nil || owner != c.name {
				c.conn.Write([]byte("You are not the owner\n"))
				continue
			}
			deleteRoom(msg)
			c.curRoom = "global"
			mu.Lock()
			clients[c.name] = c
			mu.Unlock()
			getHistory(c, c.curRoom)
			c.conn.Write([]byte("Deleted room " + msg + "\n"))

		default:
			fullMsg := strings.TrimSpace(cmd + " " + msg)
			if fullMsg != "" {
				distribute(c.name, fullMsg, c.curRoom)
				saveMessage(c.name, fullMsg, time.Now(), c.curRoom)
			}
		}
	}
}

func main() {

	config = loadConfig()

	if err := connectDatabase(); err != nil {
		log.Fatal(err)
	}

	defer closeDatabase()

	ln, err := net.Listen("tcp", ":"+config.ServerPort)
	if err != nil {
		log.Fatal("Failed to start server:", err)
	}
	defer ln.Close()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("\nShutting down...")
		ln.Close()
		mu.Lock()
		for _, client := range clients {
			client.conn.Write([]byte("Server is shutting down\n"))
			client.conn.Close()
		}
		mu.Unlock()
		closeDatabase()
		os.Exit(0)
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("Error accepting connection: %v", err)
			continue
		}
		go handleAuth(conn)
	}
}

func handleAuth(conn net.Conn) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Panic in auth: %v", r)
			conn.Close()
		}
	}()

	conn.Write([]byte("Login: "))
	login, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		conn.Close()
		return
	}
	login = strings.TrimSpace(login)

	client, err := getClient(login)

	if err == nil {
		// Existing user
		if !authenticateUser(conn, &client) {
			conn.Close()
			return
		}
		mu.Lock()
		clients[login] = client
		mu.Unlock()
		getHistory(client, client.curRoom)
		conn.Write([]byte("Welcome back to tupochat! Type /help for commands\n"))
		log.Printf("%s logged in", login)
		go handleConnection(client)
	} else {
		// New user
		if !registerUser(conn, login) {
			conn.Close()
			return
		}
	}
}

func authenticateUser(conn net.Conn, client *Client) bool {
	for attempts := 0; attempts < 3; attempts++ {
		conn.Write([]byte("Password: "))
		pswd, err := bufio.NewReader(conn).ReadString('\n')
		if err != nil {
			return false
		}
		pswd = strings.TrimSpace(pswd)

		hash := hashPassword(pswd)
		if hash == client.pswdHash {
			client.conn = conn
			return true
		}

		if attempts < 2 {
			conn.Write([]byte("Incorrect password. Try again.\n"))
		}
	}
	conn.Write([]byte("Too many failed attempts\n"))
	return false
}

func registerUser(conn net.Conn, login string) bool {
	conn.Write([]byte("Creating new account\n"))
	conn.Write([]byte("Password: "))
	pswd, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		return false
	}
	pswd = strings.TrimSpace(pswd)

	for attempts := 0; attempts < 3; attempts++ {
		conn.Write([]byte("Confirm password: "))
		confirm, err := bufio.NewReader(conn).ReadString('\n')
		if err != nil {
			return false
		}
		confirm = strings.TrimSpace(confirm)

		if confirm == pswd {
			hash := hashPassword(pswd)
			client := Client{conn, login, hash, "global"}

			if err := newClient(login, hash); err != nil {
				conn.Write([]byte("Error creating account\n"))
				return false
			}

			mu.Lock()
			clients[login] = client
			mu.Unlock()

			getHistory(client, "global")
			conn.Write([]byte("Welcome to tupochat! Type /help for commands\n"))
			log.Printf("New user registered: %s", login)
			go handleConnection(client)
			return true
		}

		if attempts < 2 {
			conn.Write([]byte("Passwords don't match. Try again.\n"))
		}
	}
	conn.Write([]byte("Too many failed attempts\n"))
	return false
}

func hashPassword(pswd string) string {
	h := sha256.New()
	h.Write([]byte(pswd))
	return hex.EncodeToString(h.Sum(nil))
}
