package main

import (
	"bufio"
	"fmt"
	"math/rand"
	"net"
	"os"
	"strconv"
	"strings"
)

type Client struct {
	conn net.Conn
	name string
	pswd string
}

var clients = make(map[string]Client)
var id = 0

func parseMessage(msg string) (string, string) {
	split := strings.Split(msg, ":")
	if len(split) == 2 {
		return split[0], split[1]
	}
	return "", split[0]
}

func distribute(from string, message string) {
	msg := fmt.Sprintf("%s: %s", from, message)
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
		cmd, msg := parseMessage(data)
		if msg == "/exit\n" {
			fmt.Printf("client %s exit chat\n", c.name)
			return
		}
		switch cmd {
		case "":
			fmt.Printf("%s: %s", c.name, msg)
			distribute(c.name, msg)
		}
	}
}

func main() {
	ln, _ := net.Listen("tcp", ":9999")
	go func() {
		reader := bufio.NewReader(os.Stdin)
		for {
			data, _ := reader.ReadString('\n')
			cmd, msg := parseMessage(data)
			switch cmd {
			case "kick":
				for k, client := range clients {
					if client.name == msg {
						fmt.Printf("kicking %s\n", msg)
						delete(clients, k)
						client.conn.Write([]byte("you are kicked\n"))
						client.conn.Close()
					}
				}
			case "id":
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
		conn.Write([]byte("name:\n"))
		name, _ := bufio.NewReader(conn).ReadString('\n')
		name = strings.TrimSuffix(name, "\n")
		id := name + "#" + strconv.Itoa(rand.Intn(9999))
		client, ok := clients[name]
		if ok {
			conn.Write([]byte("password:\n"))
			pswd, _ := bufio.NewReader(conn).ReadString('\n')
			pswd = strings.TrimSuffix(pswd, "\n")
			if pswd == client.pswd {
				conn.Write([]byte("you have login successfully\n"))
				go handleConnection(Client{conn, name, pswd})
			} else {
				conn.Write([]byte("password is incorrect\n"))
				conn.Close()
				continue
			}
		} else {
			conn.Write([]byte("password:\n"))
			pswd, _ := bufio.NewReader(conn).ReadString('\n')
			pswd = strings.TrimSuffix(pswd, "\n")
			conn.Write([]byte("confirm password:\n"))
			confirm, _ := bufio.NewReader(conn).ReadString('\n')
			confirm = strings.TrimSuffix(confirm, "\n")
			if confirm != pswd {
				conn.Write([]byte("passwords do not match\n"))
				conn.Close()
				continue
			}
			clients[id] = Client{conn, name, pswd}
			conn.Write([]byte("use your id to login\n"))
			conn.Write([]byte(id + "\n"))
			go handleConnection(Client{conn, name, pswd})
		}
	}
}
