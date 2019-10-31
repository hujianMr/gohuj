package synserver

import (
	"encoding/json"
	"fmt"
	"net/http"
)

func getLeaderUrl(w http.ResponseWriter, req *http.Request){
	_,_ = fmt.Fprint(w, leaderUrl)
}

func getServerSStatus(w http.ResponseWriter, req *http.Request){
	bytes,_ := json.Marshal(serverNodes)
	_,_ = fmt.Fprint(w, string(bytes))
}