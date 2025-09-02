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
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

var db *pgx.Conn

type Client struct {
	conn     net.Conn
	login    string
	name     string
	pswdHash string
}

func connectDatabase() {
	var err error
	db, err = pgx.Connect(context.Background(), "postgres://artemloginov:raw0@localhost:5432/tupochatdb")
	if err != nil {
		log.Fatal("Failed to conect to database: ", err)
	}
}

func closeDatabase() {
	db.Close(context.Background())
}

func newClient(name string, pswd string) {
	var err error
	_, err = db.Exec(context.Background(), "INSERT INTO clients (username, display_name, password_hash) VALUES ($1, $2, $3)", name, name, pswd)
	if err != nil {
		log.Fatal("Failed to insert client: ", err)
	}
}

func getClient(login string) Client {
	var client Client
	err := db.QueryRow(context.Background(), "SELECT username, display_name, password_hash FROM clients WHERE username = $1", login).Scan(&client.login, &client.name, &client.pswdHash)
	if err != nil {
		log.Fatal("Failed to get client: ", err)
	}
	return client
}

func saveMessage(from string, message string) {
	var err error
	var id string
	err = db.QueryRow(context.Background(), "SELECT id FROM clients WHERE username = $1", from).Scan(&id)
	if err != nil {
		log.Fatal("Failed to get client id: ", err)
	}
	_, err = db.Exec(context.Background(), "INSERT INTO messages (sender_id, content) VALUES ($1, $2)", id, message)
	if err != nil {
		log.Fatal("Failed to insert message: ", err)
	}
}

var clients = make(map[string]Client)

func parseMessage(msg string) (string, string) {
	split := strings.Split(msg, " ")
	if len(split) == 1 {
		return "", split[0]
	}
	return split[0], strings.Join(split[1:], " ")
}

func distribute(from string, message string) {
	time := time.Now().Format("2006-01-02 15:04:05")
	msg := fmt.Sprintf("%s %s: %s\n", time, from, message)
	for _, client := range clients {
		client.conn.Write([]byte(msg))
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
		if cmd == "/help" {
			fmt.Printf("commands:\n/help - this help\n/name - change your name\n/exit - exit chat\n")
			continue
		}
		if cmd == "/name" {
			if msg != "" || len(msg) <= 20 {
				c.name = msg
				clients[c.login] = c
				_, err := db.Exec(context.Background(), "UPDATE clients SET display_name = $1 WHERE username = $2", msg, c.login)
				if err != nil {
					log.Fatal("Failed to update client: ", err)
				}
				fmt.Printf("%s changed name to %s\n", c.name, msg)
				continue
			}
			c.conn.Write([]byte("name must be between 1 and 20 characters\n"))
		} else if strings.TrimSuffix(cmd+msg, "\n") != "" {
			fmt.Printf("%s %s: %s\n", time.Now().Format("2006-01-02 15:04:05"), c.name, cmd+" "+msg)
			distribute(c.name, cmd+" "+msg)
			saveMessage(c.login, cmd+" "+msg)
		}
	}
}

func main() {
	ln, _ := net.Listen("tcp", ":9999")
	connectDatabase()
	defer closeDatabase()
	go func() {
		reader := bufio.NewReader(os.Stdin)
		for {
			data, _ := reader.ReadString('\n')
			fmt.Print("\033[1A")
			fmt.Print("\033[2K")
			cmd, msg := parseMessage(data)
			if cmd == "/help" {
				fmt.Printf("commands:\n/help - this help\n/kick id - kick client with id\n/id name - get id of client with name\n")
				continue
			}
			if cmd == "/kick" {
				client, ok := clients[msg]
				if ok {
					fmt.Printf("kicked %s\n", client.name)
					client.conn.Close()
				}
			} else if strings.TrimSuffix(cmd+msg, "\n") != "" {
				fmt.Printf("%s server: %s", time.Now().Format("2006-01-02 15:04:05"), cmd+" "+msg)
				distribute("server", cmd+" "+msg)
			}
		}
	}()
	for {
		conn, _ := ln.Accept()
		conn.Write([]byte("login:\n"))
		login, _ := bufio.NewReader(conn).ReadString('\n')
		login = strings.TrimSuffix(login, "\n")
		var registered bool
		err := db.QueryRow(context.Background(), "SELECT EXISTS( SELECT 1 FROM clients WHERE username = $1)", login).Scan(&registered)
		if err != nil {
			log.Fatal("Failed to check if user is registered: ", err)
		}
		if registered {
			client := getClient(login)
			conn.Write([]byte("password:\n"))
			pswd, _ := bufio.NewReader(conn).ReadString('\n')
			pswd = strings.TrimSuffix(pswd, "\n")
			sh := sha256.New()
			sh.Write([]byte(pswd))
			hash := hex.EncodeToString(sh.Sum(nil))
			if hash == client.pswdHash {
				conn.Write([]byte("welcome to chat\n"))
				client.conn = conn
				clients[login] = client
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
						conn.Write([]byte("welcome to chat\n"))
						client.conn = conn
						clients[login] = client
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
						conn.Write([]byte("welcome to chat\n"))
						sh.Write([]byte(pswd))
						hash := hex.EncodeToString(sh.Sum(nil))
						clients[login] = Client{conn, login, login, hash}
						newClient(login, hash)
						go handleConnection(Client{conn, login, login, hash})
						break
					}
				}
				conn.Close()
				continue
			}
			sh.Write([]byte(pswd))
			hash := hex.EncodeToString(sh.Sum(nil))
			clients[login] = Client{conn, login, login, hash}
			newClient(login, hash)
			go handleConnection(Client{conn, login, login, hash})
		}
	}
}
