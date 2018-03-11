package main

import (
	"fmt"
	"math/rand"
	"net/http"
	"sync"

	"encoding/json"
	"github.com/googollee/go-socket.io"
)

var connect4WaitingRoom connect4Room

const (
	connect4Name = "connect4"
)

type connect4Room struct {
	uuid string
	m    *sync.Mutex
}

var currentGames games

type connect4Game struct {
	P1           string     `json:"player1"`
	P2           string     `json:"player2"`
	Grid         [][]string `json:"grid"`
	Uuid         string     `json:"uuid"`
	Turn         string     `json:"turn"`
	Winner       string     `json:"winner"`
	LastPlayCol  int64      `json:"last_play_col"`
	LastPlayLine int64      `json:"last_play_line"`
}

type games struct {
	m     *sync.Mutex
	games map[string]connect4Game
}

func init() {
	connect4WaitingRoom = connect4Room{
		uuid: "",
		m:    &sync.Mutex{},
	}
	currentGames = games{
		m:     &sync.Mutex{},
		games: make(map[string]connect4Game),
	}
}

func joinConnect4(c socketio.Socket, uuid string) string {
	resp := response{}
	connect4WaitingRoom.m.Lock()
	defer connect4WaitingRoom.m.Unlock()
	connectedUser := connect4WaitingRoom.uuid

	if connectedUser == "" {
		return saveInConnect4WaitingRoom(c, resp, uuid)
	}
	if connectedUser == uuid {
		resp.Status = http.StatusForbidden
		return writeResponse(resp)
	}
	connect4WaitingRoom.uuid = ""

	// Create game with the 2 users
	gameUUID, errU := generateUUID()
	if errU != nil {
		resp.Status = http.StatusInternalServerError
		resp.Error = fmt.Sprintf("joinConnect4> Cannot generate game uuid: %s", errU)
		return writeResponse(resp)
	}

	grid := make([][]string, launchpadSize)
	for i := range grid {
		grid[i] = make([]string, launchpadSize)
	}
	newGame := connect4Game{
		Grid:         grid,
		P1:           uuid,
		P2:           connectedUser,
		Uuid:         gameUUID,
		LastPlayLine: -1,
		LastPlayCol:  -1,
	}
	i := rand.Intn(2)
	if i == 0 {
		newGame.Turn = newGame.P1
	} else {
		newGame.Turn = newGame.P2
	}

	currentGames.m.Lock()
	currentGames.games[gameUUID] = newGame
	currentGames.m.Unlock()

	resp.Status = http.StatusCreated
	resp.Data = newGame
	resp.Game = connect4Name
	resp.GameUUID = gameUUID

	respString := writeResponse(resp)
	// send notif to players
	user1, _ := readUserMap(newGame.P1)
	user2, b := readUserMap(newGame.P2)
	if !b {
		return saveInConnect4WaitingRoom(c, resp, uuid)
	}
	user1.so.Emit(eventPlay, respString)
	user2.so.Emit(eventPlay, respString)
	return respString
}

func saveInConnect4WaitingRoom(c socketio.Socket, resp response, uuid string) string {
	connect4WaitingRoom.uuid = uuid
	resp.Status = http.StatusAccepted
	c.Leave(waitingRoom)
	c.BroadcastTo(waitingRoom, "eventConnect4Waiting", nil)
	return writeResponse(resp)
}

type playConnect4Request struct {
	GameUUID string `json:"game_uuid"`
	UserUUID string `json:"user_uuid"`
	Col      int64  `json:"col"`
}

func playConnect4(data string) string {
	resp := response{}

	var r playConnect4Request
	if err := json.Unmarshal([]byte(data), &r); err != nil {
		resp.Status = http.StatusBadRequest
		resp.Error = fmt.Sprintf("playConnect4> bad request: %s: %s", data, err)
		return writeResponse(resp)
	}

	// get game
	var g connect4Game
	var b bool
	g, b = currentGames.games[r.GameUUID]
	if !b {
		resp.Status = http.StatusNotFound
		resp.Error = "playConnect4> Game not found"
		return writeResponse(resp)
	}
	if g.Turn != r.UserUUID {
		resp.Status = http.StatusForbidden
		resp.Error = "playConnect4> you can't play"
		return writeResponse(resp)
	}

	// check col
	if g.Grid[r.Col][launchpadSize-1] != "" {
		resp.Status = http.StatusForbidden
		resp.Error = "playConnect4> invalid column"
		return writeResponse(resp)
	}

	// play
	var currentPlayLine int64
	for i := 2; i < 9; i++ {
		if g.Grid[r.Col][launchpadSize-i] != "" {
			g.Grid[r.Col][launchpadSize-i+1] = r.UserUUID
			currentPlayLine = launchpadSize - int64(i) + 1
			break
		}
		if i == launchpadSize && g.Grid[r.Col][0] == "" {
			currentPlayLine = 0
			g.Grid[r.Col][0] = r.UserUUID
		}
	}
	g.LastPlayCol = r.Col
	g.LastPlayLine = currentPlayLine

	// check end of game
	hasWin := horizontalCheck(g.Grid, r.Col, currentPlayLine)
	if !hasWin {
		hasWin = verticalCheck(g.Grid, r.Col, currentPlayLine)
		if !hasWin {
			hasWin = diagLeftRightCheck(g.Grid, r.Col, currentPlayLine)
			if !hasWin {
				hasWin = diagRightLeftCheck(g.Grid, r.Col, currentPlayLine)
			}
		}
	}

	// next player to play
	if g.Turn == g.P1 {
		g.Turn = g.P2
	} else {
		g.Turn = g.P1
	}

	// check victory
	if hasWin {
		g.Winner = r.UserUUID
	}

	resp.Data = g

	// Update game
	currentGames.m.Lock()
	currentGames.games[r.GameUUID] = g
	currentGames.m.Unlock()

	resp.Game = connect4Name
	resp.GameUUID = g.Uuid

	respString := writeResponse(resp)

	p1, b1 := readUserMap(g.P1)
	p2, b2 := readUserMap(g.P2)
	if !b1 && b2 {
		fmt.Println("FORFAIT")
		g.Winner = p2.uuid
		resp.Data = g
		p2.so.Emit(eventPlay, respString)
		respString = writeResponse(resp)
		return respString
	} else if b1 && !b2 {
		fmt.Println("FORFAIT2")
		g.Winner = p1.uuid
		resp.Data = g
		p1.so.Emit(eventPlay, respString)
		respString = writeResponse(resp)
		return respString
	}
	p1.so.Emit(eventPlay, respString)
	p2.so.Emit(eventPlay, respString)

	// send game updated
	return respString
}

func horizontalCheck(grid [][]string, col int64, line int64) bool {
	currentRight := col + 1
	currentLeft := col - 1

	uuid := grid[col][line]

	count := 1
	// check left side
	for {
		if currentLeft == -1 || grid[currentLeft][line] != uuid {
			break
		}
		count++
		currentLeft--
	}
	for {
		if currentRight == launchpadSize || grid[currentRight][line] != uuid {
			break
		}
		count++
		currentRight++
	}
	return count >= 4
}

func verticalCheck(grid [][]string, col int64, line int64) bool {
	if line < 3 {
		return false
	}
	uuid := grid[col][line]

	count := 1
	currentLine := line - 1
	for {
		if currentLine < 0 || grid[col][currentLine] != uuid {
			break
		}
		count++
		currentLine--
	}
	return count >= 4
}

func diagLeftRightCheck(grid [][]string, col int64, line int64) bool {
	uuid := grid[col][line]

	currentLine := line + 1
	currentCol := col - 1
	count := 1

	for {
		if currentLine == launchpadSize || currentCol < 0 || grid[currentCol][currentLine] != uuid {
			break
		}
		count++
		currentLine++
		currentCol--
	}

	currentLine = line - 1
	currentCol = col + 1

	for {
		if currentCol == launchpadSize || currentLine < 0 || grid[currentCol][currentLine] != uuid {
			break
		}
		count++
		currentLine--
		currentCol++
	}

	return count >= 4
}

func diagRightLeftCheck(grid [][]string, col int64, line int64) bool {
	uuid := grid[col][line]

	currentLine := line + 1
	currentCol := col + 1
	count := 1

	for {
		if currentLine == launchpadSize || currentCol == launchpadSize || grid[currentCol][currentLine] != uuid {
			break
		}
		count++
		currentLine++
		currentCol++
	}

	currentLine = line - 1
	currentCol = col - 1

	for {
		if currentCol < 0 || currentLine < 0 || grid[currentCol][currentLine] != uuid {
			break
		}
		count++
		currentLine--
		currentCol--
	}

	return count >= 4
}
