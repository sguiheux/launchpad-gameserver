package main

import (
	"fmt"
	"github.com/googollee/go-socket.io"
	"net/http"
	"sync"
	"time"
)

var userMap map[string]User
var userMutex *sync.Mutex

type User struct {
	uuid      string `json:"uuid"`
	so        socketio.Socket
	heartbeat time.Time
}

func init() {
	userMap = make(map[string]User)
	userMutex = &sync.Mutex{}
}

func heartbeat(uuid string) string {
	resp := response{}
	if uuid == "" {
		resp.Status = http.StatusUnauthorized
		return writeResponse(resp)
	}
	u, ok := userMap[uuid]
	if !ok {
		resp.Status = http.StatusUnauthorized
		return writeResponse(resp)
	}
	u.heartbeat = time.Now()

	userMutex.Lock()
	userMap[u.uuid] = u
	userMutex.Unlock()

	resp.Status = http.StatusOK
	resp.Data = uuid
	return writeResponse(resp)
}

func connect(so socketio.Socket) string {
	resp := response{}
	// register new user
	var errUUID error
	uuid, errUUID := generateUUID()
	if errUUID != nil {
		resp.Error = fmt.Sprintf("heartbeat> cannot generate uuid: %s", errUUID)
		resp.Status = http.StatusInternalServerError
		return writeResponse(resp)
	}

	u := User{
		uuid:      uuid,
		heartbeat: time.Now(),
		so:        so,
	}
	userMutex.Lock()
	userMap[u.uuid] = u
	userMutex.Unlock()

	resp.Data = u.uuid
	resp.Status = http.StatusOK
	return writeResponse(resp)

}
