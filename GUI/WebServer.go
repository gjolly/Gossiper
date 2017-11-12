package GUI

import (
	"net"
	"github.com/gorilla/mux"
	"net/http"
	"io/ioutil"
	"fmt"
	"encoding/json"
	"../tools"
)

type WebServer struct {
	conn *net.UDPConn
	Addr *net.UDPAddr

	sendMsg  func(string)
	messages *map[string](map[uint32]tools.RumorMessage)
}

func NewWebServer(servAddr string, sendMsg func(string), messages *map[string](map[uint32]tools.RumorMessage)) (ws *WebServer) {
	addr, err := net.ResolveUDPAddr("udp4", servAddr)
	if err != nil {
		panic(err)
	}
	conn, err := net.ListenUDP("udp4", addr)
	return &WebServer{conn, addr, sendMsg, messages}
}

func (ws WebServer) Run() {
	r := mux.NewRouter()
	r.HandleFunc("/sendMsg", ws.sendMessage)
	r.HandleFunc("/messReceived", ws.messageReceived)
	r.HandleFunc("/", ws.sendPage)

	http.ListenAndServe(ws.Addr.String(), r)
}

func (ws WebServer) sendPage(response http.ResponseWriter, request *http.Request) {
	file, err := ioutil.ReadFile("../GUI/gui.html")
	if err != nil {
		fmt.Println("WebServer: failed to open gui.html")
	}
	response.Write(file)
}

func (ws WebServer) sendMessage(response http.ResponseWriter, request *http.Request) {
	message := request.PostFormValue("mess")
	fmt.Println("WebServer: mess to send = ", message)
	ws.sendMsg(message)
}

func (ws WebServer) messageReceived(response http.ResponseWriter, request *http.Request) {
	messages, err := json.Marshal(ws.messages)
	if err != nil {
		fmt.Println(err)
	}
	response.Write(messages)
}
