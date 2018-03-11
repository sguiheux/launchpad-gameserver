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
	eventPlay   = "play"
)

type response struct {
	Status   int         `json:"status"`
	Data     interface{} `json:"data"`
	Error    string      `json:"error"`
	Game     string      `json:"game"`
	GameUUID string      `json:"game_uuid"`
}

type joinRequest struct {
	Game string `json:"game"`
	UUID string `json:"uuid""`
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
		so.On("join", func(data string) string {
			fmt.Println("join", data)
			return joinGame(so, data)
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

func joinGame(so socketio.Socket, data string) string {
	resp := response{}
	var r joinRequest
	if err := json.Unmarshal([]byte(data), &r); err != nil {
		resp.Status = http.StatusBadRequest
		resp.Error = fmt.Sprintf("joinGame> Unable to unmarhsall %s", data)
		return writeResponse(resp)
	}
	switch r.Game {
	case connect4Name:
		return joinConnect4(so, r.UUID)
	}

	resp.Status = http.StatusBadRequest
	resp.Error = fmt.Sprintf("Unknown game %s", r.Game)
	return writeResponse(resp)
}

func writeResponse(resp response) string {
	b, _ := json.Marshal(resp)
	if resp.Status >= 400 {
		fmt.Println(string(b))
	}
	return string(b)
}
