package GUI

import (
	"net"
	"github.com/gorilla/mux"
	"net/http"
	"io/ioutil"
	"fmt"
	"encoding/json"
	"github.com/gjolly/Gossiper/tools"
	"github.com/gjolly/Gossiper/tools/Messages"
)

type WebServer struct {
	conn *net.UDPConn
	Addr *net.UDPAddr

	sendMsg         func(string)
	sendPrivateMsg  func(string, string)
	share 			func(string)
	messages        tools.ListMessages
	privateMessages *[]Messages.PrivateMessage
	routingTable    *tools.RoutingTable
}

func NewWebServer(servAddr string, sendMsg func(string), share func(string),sendPrivateMsg func(string, string), messages tools.ListMessages, privateMessages *[]Messages.PrivateMessage, routingTable *tools.RoutingTable) (ws *WebServer, ) {

	addr, err := net.ResolveUDPAddr("udp4", servAddr)
	if err != nil {
		panic(err)
	}
	conn, err := net.ListenUDP("udp4", addr)
	return &WebServer{conn, addr, sendMsg, sendPrivateMsg, share,
		messages, privateMessages, routingTable}
}

func (ws WebServer) Run() {
	r := mux.NewRouter()
	r.HandleFunc("/sendMsg", ws.sendMessage)
	r.HandleFunc("/sendPrivate", ws.sendPrivate)
	r.HandleFunc("/messReceived", ws.messageReceived)
	r.HandleFunc("/getPrivateMessages", ws.getPrivateMessages)
	r.HandleFunc("/nodes", ws.nodes)
	r.HandleFunc("/shareFile", ws.shareFile)
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
	ws.messages.Mutex.Lock()
	messages, err := json.Marshal(ws.messages.Messages)
	ws.messages.Mutex.Unlock()
	if err != nil {
		fmt.Println(err)
	}
	response.Write(messages)
}

func (ws WebServer) getPrivateMessages(response http.ResponseWriter, request *http.Request) {
	messages, err := json.Marshal(*ws.privateMessages)
	if err != nil {
		fmt.Println(err)
	}
	response.Write(messages)
}

func (ws WebServer) nodes(response http.ResponseWriter, request *http.Request) {
	messages, err := json.Marshal(ws.routingTable.GetTable())
	if err != nil {
		fmt.Println(err)
	}
	response.Write(messages)
}

func (ws WebServer) sendPrivate(response http.ResponseWriter, request *http.Request) {
	message := request.PostFormValue("mess")
	node := request.PostFormValue("node")
	fmt.Println("WebServer: private mess to send = ", node, message)
	ws.sendPrivateMsg(message, node)
}

func (ws WebServer) shareFile(response http.ResponseWriter, request *http.Request) {
	file := request.PostFormValue("fileShared")
	ws.share(file)
}
