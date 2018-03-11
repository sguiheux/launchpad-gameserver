package main

import (
	"encoding/json"

	"fmt"
	"github.com/googollee/go-socket.io"
	"log"
	"net/http"
)

const (
	launchpadSize = 8

	waitingRoom = "waiting.room"
	eventPlay = "play"
)

type response struct {
	Status int         `json:"status"`
	Data   interface{} `json:"data"`
	Error  string      `json:"error"`
	Game   string    `json:"game"`
	GameUUID string  `json:"game_uuid"`
}

func main() {
	//create
	server, err := socketio.NewServer(nil)
	if err != nil {
		log.Fatal(err)
	}

	//handle connected
	server.On("connection", func(so socketio.Socket) {
		so.Emit("connect", connect(so))

		//join them to room
		so.Join(waitingRoom)

		so.On("heartbeat", func(uuid string) string {
			return heartbeat(uuid)
		})
		so.On("join.connect4", func(uuid string) string {
			return joinConnect4(so, uuid)
		})
		so.On("play.connect4", func(data string) string {
			return playConnect4(data)
		})
	})

	server.On("error", func(so socketio.Socket, err error) {
		log.Println("error:", err)
	})

	http.Handle("/socket.io/", server)
	http.Handle("/", http.FileServer(http.Dir("./client")))
	log.Println("Serving at localhost:5000...")
	log.Fatal(http.ListenAndServe(":5000", nil))
}

func writeResponse(resp response) string {
	b, _ := json.Marshal(resp)
	if resp.Status >= 400 {
		fmt.Println(string(b))
	}
	return string(b)
}
