package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"
)

type Client struct {
	conn net.Conn
	pswd string
	name string
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
	msg := fmt.Sprintf("%s: %s\n", from, message)
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
				clients[c.name] = c
				fmt.Printf("%s changed name to %s\n", c.name, msg)
				continue
			}
			fmt.Printf("name must be between 1 and 20 characters\n")
		} else if strings.TrimSuffix(cmd + msg, "\n") != "" {
			fmt.Printf("%s: %s\n", c.name, cmd + msg)
			distribute(c.name, cmd + msg)
		}
	}
}

func main() {
	ln, _ := net.Listen("tcp", ":9999")
	go func() {
		reader := bufio.NewReader(os.Stdin)
		for {
			data, _ := reader.ReadString('\n')
			fmt.Print("\033[1A")
			fmt.Print("\033[2K")
			cmd, msg := parseMessage(data)
			switch cmd {
			case "/help":
				fmt.Printf("commands:\n/help - this help\n/kick id - kick client with id\n/id name - get id of client with name\n")
			case "/kick":
				client, ok := clients[msg]
				if ok {
					fmt.Printf("kicked %s\n", client.name)
					client.conn.Close()
				}
			case "/id":
				for k, client := range clients {
					if client.name == msg {
						fmt.Printf("id is %s\n", k)
					}
				}
			case "":
				fmt.Printf("server: %s", msg)
				distribute("server", msg)
			}
		}
	}()
	for {
		conn, _ := ln.Accept()
		conn.Write([]byte("login:\n"))
		login, _ := bufio.NewReader(conn).ReadString('\n')
		login = strings.TrimSuffix(login, "\n")
		client, ok := clients[login]
		if ok {
			conn.Write([]byte("password:\n"))
			pswd, _ := bufio.NewReader(conn).ReadString('\n')
			pswd = strings.TrimSuffix(pswd, "\n")
			if pswd == client.pswd {
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
					if pswd == client.pswd {
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
						clients[login] = Client{conn, pswd, login}
						go handleConnection(Client{conn, pswd, login})
						break
					}
				}
				conn.Close()
				continue
			}
			clients[login] = Client{conn, pswd, login}
			go handleConnection(Client{conn, pswd, login})
		}
	}
}
