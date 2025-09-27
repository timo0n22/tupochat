package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

var db *pgx.Conn
var clients = make(map[string]Client)

type Client struct {
	conn     net.Conn
	name     string
	pswdHash string
	curRoom  string
}

func connectDatabase(test bool) {
	var err error

	if test {
		db, err = pgx.Connect(context.Background(), "postgres://test:pass@localhost:5432/tupochatdb")
		if err != nil {
			log.Fatal("Failed to conect to database: ", err)
		}
		fmt.Println("Connected to test database")
		return
	}

	db, err = pgx.Connect(context.Background(), "postgres://postgres:raw0@localhost:5432/tupochatdb")
	if err != nil {
		log.Fatal("Failed to conect to database: ", err)
	}
	fmt.Println("Connected to database")
}

func closeDatabase() {
	db.Close(context.Background())
}

func newClient(name string, pswd string) {
	var err error
	_, err = db.Exec(context.Background(), "INSERT INTO clients (username, password_hash) VALUES ($1, $2)", name, pswd)
	if err != nil {
		log.Fatal("Failed to insert client: ", err)
	}
}

func getClient(name string) (Client, error) {
	var client Client
	err := db.QueryRow(context.Background(), "SELECT username, password_hash, current_room FROM clients WHERE username = $1", name).Scan(&client.name, &client.pswdHash, &client.curRoom)
	if err != nil {
		log.Println("Failed to get client: ", err)
	}
	return client, err
}

func saveMessage(from string, message string, time time.Time, room string) {
	var err error
	timestamp := time.Format("2006-01-02 15:04:05")
	_, err = db.Exec(context.Background(), "INSERT INTO messages (sender, content, sent_at, room) VALUES ($1, $2, $3, $4)", from, message, timestamp, room)
	if err != nil {
		log.Fatal("Failed to insert message: ", err)
	}
}

func checkRoom(name string) (string, error) {
	var owner string
	err := db.QueryRow(context.Background(), "SELECT owner FROM rooms WHERE name = $1", name).Scan(&owner)
	if err != nil {
		return "", err
	}
	return owner, nil
}

func newRoom(name string, owner string) (string, error) {
	var exists bool
	err := db.QueryRow(context.Background(), "SELECT EXISTS( SELECT 1 FROM rooms WHERE name = $1)", name).Scan(&exists)
	if err != nil {
		log.Fatal("Failed to check if room exists: ", err)
	}
	if exists {
		return "room " + name + " already exists\n", err
	}
	_, err = db.Exec(context.Background(), "INSERT INTO rooms (name, owner) VALUES ($1, $2)", name, owner)
	if err != nil {
		log.Fatal("Failed to create room: ", err)
	}
	_, err = db.Exec(context.Background(), "UPDATE clients SET current_room = $1 WHERE username = $2", name, owner)
	if err != nil {
		log.Fatal("Failed to join room: ", err)
	}
	return "created room " + name + "\n", nil
}

func getHistory(client Client, room string) {
	rows, err := db.Query(context.Background(),
		"SELECT content, sent_at, sender FROM messages WHERE room = $1 ORDER BY sent_at ASC LIMIT 500", room)
	if err != nil {
		log.Fatal("Failed to get history: ", err)
	}
	defer rows.Close()

	var history []string
	var names []string
	var timestamps []string

	for rows.Next() {
		var msg string
		var name string
		var ts string
		if err := rows.Scan(&msg, &ts, &name); err != nil {
			log.Println("Failed to scan row:", err)
			continue
		}
		history = append(history, msg)
		timestamps = append(timestamps, ts)
		names = append(names, name)
	}

	if err := rows.Err(); err != nil {
		log.Fatal("Row iteration error: ", err)
	}

	for i, msg := range history {
		client.conn.Write([]byte(timestamps[i] + " " + names[i] + ": " + msg + "\n"))
	}
}

func listRooms() ([]string, error) {
	var rooms []string
	err := db.QueryRow(context.Background(),
		"SELECT name FROM rooms").Scan(&rooms)
	if err != nil {
		log.Fatal("Failed to get rooms: ", err)
		return nil, err
	}
	return rooms, nil
}

func roomExists(name string) bool {
	var roomExists bool
	err := db.QueryRow(context.Background(), "SELECT EXISTS( SELECT 1 FROM rooms WHERE name = $1)", name).Scan(&roomExists)
	if err != nil {
		log.Fatal("Failed to check if room exists: ", err)
		//todo return error instead of fatal exit
	}
	return roomExists
}

func joinRoom(name string, room string) {
	_, err := db.Exec(context.Background(), "UPDATE clients SET current_room = $1 WHERE username = $2", room, name)
	if err != nil {
		log.Fatal("Failed to join room: ", err)
	}
}

func deleteRoom(name string) {
	_, err := db.Exec(context.Background(), "UPDATE clients SET current_room = 'global' WHERE current_room = $1", name)
	if err != nil {
		log.Fatal("Failed to delete room in clients: ", err)
	}
	_, err = db.Exec(context.Background(), "DELETE FROM rooms WHERE name = $1", name)
	if err != nil {
		log.Fatal("Failed to delete room: ", err)
	}
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

	for _, client := range clients {
		if client.curRoom == room {
			client.conn.Write([]byte(msg))
		}
	}
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

		if cmd == "/exit\n" {
			fmt.Printf("client %s exit chat\n", c.name)
			return
		}

		if cmd == "/help\n" {
			c.conn.Write([]byte("commands:\n/room - create and join room\n/join - join room\n/deleteRoom - delete room\n/list - list rooms\n/exit - exit chat\n"))
			continue
		}

		if cmd == "/list\n" {
			rooms, err := listRooms()
			if err != nil {
				log.Fatal("Failed to list rooms: ", err)
			}
			for _, room := range rooms {
				c.conn.Write([]byte(room + "\n"))
			}
			continue
		}

		if cmd == "/room" {
			if msg == "" || len(msg) > 20 {
				c.conn.Write([]byte("room name must be between 1 and 20 characters\n"))
				continue
			}
			if roomExists(msg) {
				c.conn.Write([]byte("room " + msg + " already exists\n"))
				continue
			}
			result, err := newRoom(msg, c.name)
			if err == nil {
				c.curRoom = msg
				clients[c.name] = c
			}
			fmt.Println(result)
			c.conn.Write([]byte(result))
			continue
		}

		if cmd == "/join" {
			if msg == "" || len(msg) > 20 {
				c.conn.Write([]byte("room name must be between 1 and 20 characters\n"))
			}
			if !roomExists(msg) {
				c.conn.Write([]byte("room " + msg + " does not exist\n"))
				continue
			}
			joinRoom(c.name, msg)
			c.curRoom = msg
			clients[c.name] = c
			getHistory(c, c.curRoom)
			fmt.Printf("%s joined room %s\n", c.name, msg)
			c.conn.Write([]byte("joined " + msg + "\n"))
			continue
		}

		if cmd == "/deleteRoom" {
			if msg == "" || len(msg) > 20 {
				c.conn.Write([]byte("room name must be between 1 and 20 characters\n"))
			}
			if !roomExists(msg) {
				c.conn.Write([]byte("room " + msg + " does not exist\n"))
				continue
			}
			owner, err := checkRoom(msg)
			if err != nil {
				log.Fatal("Failed to get room: ", err)
			}
			if owner != c.name {
				c.conn.Write([]byte("you are not the owner of this room\n"))
				continue
			}
			joinRoom(c.name, "global")
			deleteRoom(msg)
			c.curRoom = "global"
			clients[c.name] = c
			getHistory(c, c.curRoom)
			fmt.Printf("%s deleted room %s\n", c.name, msg)
			c.conn.Write([]byte("deleted room " + msg + "\n"))
			continue
		} else if strings.TrimSuffix(cmd+msg, "\n") != "" {
			fmt.Printf("%s --%s -- %s: %s\n", c.curRoom, time.Now().Format("2006-01-02 15:04:05"), c.name, cmd+" "+msg)
			distribute(c.name, cmd+" "+msg, c.curRoom)
			saveMessage(c.name, cmd+" "+msg, time.Now(), c.curRoom)
		}
	}
}

func main() {
	ln, _ := net.Listen("tcp", ":5522")
	testPtr := flag.Bool("test", false, "test")
	flag.Parse()
	serverRoom := "global"

	connectDatabase(*testPtr)
	defer closeDatabase()

	go func() {
		reader := bufio.NewReader(os.Stdin)
		for {
			data, _ := reader.ReadString('\n')
			fmt.Print("\033[1A")
			fmt.Print("\033[2K")
			cmd, msg := parseMessage(data)
			if cmd == "/help" {
				fmt.Printf("commands:\n/help - this help\n")
				continue
			} else if strings.TrimSuffix(cmd+msg, "\n") != "" {
				fmt.Printf("%s server: %s", time.Now().Format("2006-01-02 15:04:05"), cmd+" "+msg)
				saveMessage("server", strings.TrimSuffix(cmd+" "+msg, "\n"), time.Now(), serverRoom)
				distribute("server", strings.TrimSuffix(cmd+" "+msg, "\n"), serverRoom)
			}
		}
	}()
	for {
		conn, _ := ln.Accept()
		conn.Write([]byte("login:\n"))
		login, _ := bufio.NewReader(conn).ReadString('\n')
		login = strings.TrimSuffix(login, "\n")
		client, err := getClient(login)

		if err == nil {
			conn.Write([]byte("password:\n"))
			pswd, _ := bufio.NewReader(conn).ReadString('\n')
			pswd = strings.TrimSuffix(pswd, "\n")
			sh := sha256.New()
			sh.Write([]byte(pswd))
			hash := hex.EncodeToString(sh.Sum(nil))
			if hash == client.pswdHash {
				client.conn = conn
				clients[login] = client
				getHistory(client, client.curRoom)
				conn.Write([]byte("welcome to tupochat! type /help to see commands\n"))
				go handleConnection(client)
			} else {
				conn.Write([]byte("password is incorrect\n"))
				for i := 0; i < 2; i++ {
					conn.Write([]byte("try again\n"))
					conn.Write([]byte("password:\n"))
					pswd, _ := bufio.NewReader(conn).ReadString('\n')
					pswd = strings.TrimSuffix(pswd, "\n")
					sh.Write([]byte(pswd))
					hash := hex.EncodeToString(sh.Sum(nil))
					if hash == client.pswdHash {
						client.conn = conn
						clients[login] = client
						getHistory(client, client.curRoom)
						conn.Write([]byte("welcome to tupochat! type /help to see commands\n"))
						go handleConnection(client)
						break
					}
				}
				conn.Write([]byte("connection closed\n"))
				conn.Close()
				continue
			}
		} else {
			conn.Write([]byte("login not found, creating new user\n"))
			conn.Write([]byte("password:\n"))
			pswd, _ := bufio.NewReader(conn).ReadString('\n')
			pswd = strings.TrimSuffix(pswd, "\n")
			sh := sha256.New()
			conn.Write([]byte("confirm password:\n"))
			confirm, _ := bufio.NewReader(conn).ReadString('\n')
			confirm = strings.TrimSuffix(confirm, "\n")
			if confirm != pswd {
				conn.Write([]byte("passwords do not match, try again\n"))
				for i := 0; i < 2; i++ {
					conn.Write([]byte("confirm password:\n"))
					confirm, _ := bufio.NewReader(conn).ReadString('\n')
					confirm = strings.TrimSuffix(confirm, "\n")
					if confirm == pswd {
						sh.Write([]byte(pswd))
						hash := hex.EncodeToString(sh.Sum(nil))
						clients[login] = Client{conn, login, hash, "global"}
						newClient(login, hash)
						getHistory(Client{conn, login, hash, "global"}, "global")
						conn.Write([]byte("welcome to tupochat! type /help to see commands\n"))
						go handleConnection(Client{conn, login, hash, "global"})
						break
					}
				}
				conn.Close()
				continue
			}
			sh.Write([]byte(pswd))
			hash := hex.EncodeToString(sh.Sum(nil))
			clients[login] = Client{conn, login, hash, "global"}
			newClient(login, hash)
			getHistory(Client{conn, login, hash, "global"}, "global")
			conn.Write([]byte("welcome to tupochat! type /help to see commands\n"))
			go handleConnection(Client{conn, login, hash, "global"})
		}
	}
}
