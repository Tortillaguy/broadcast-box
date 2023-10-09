package state

import (
	"sync"
)


var payload = ""
var payloadLock sync.Mutex

func updatePayload(data string){
	payloadLock.Lock()
	defer payloadLock.Unlock()
	payload = data
}