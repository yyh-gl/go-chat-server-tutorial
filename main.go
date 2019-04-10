package main

import (
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

// mapでwebsocketのコネクションに対するポインタを定義
var clients = make(map[*websocket.Conn]bool)
// クライアント側から送られて来たメッセージをキューイングする役目
var broadcast = make(chan Message)
// HTTPコネクションを受け取ってwebsocketにアップグレードする関数を保持したオブジェクト
var upgrader = websocket.Upgrader{}

// メッセージを保持するための構造体
// `json:"xxx"` という記述は JSONにおける対応キーを指定している
type Message struct {
	Email    string `json:"email"`
	Username string `json:"username"`
	Message  string `json:"message"`
}

func main() {
	// 静的ファイルを参照するファイルサーバの立ち上げやルーティングの紐づけ
	fs := http.FileServer(http.Dir("./public"))
	http.Handle("/", fs)

	// websocketの接続を始めるためのリクエストをハンドリング
	http.HandleFunc("/ws", handleConnections)
	// goroutine スタート！
	go handleMessages()

	log.Println("http server started on :8000")
	err := http.ListenAndServe(":8000", nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}

// コネクションハンドラ
// [処理フロー]
// 1. GETリクエストをwebsocketにアップグレード
// 2. 受け取ったリクエストをクライアントとして登録
// 3. websocketからのメッセージを待機
// 4. メッセージを受信するとbroadcastチャネルに送信
func handleConnections(w http.ResponseWriter, r *http.Request) {
	// 1. GETリクエストをwebsocketにアップグレード
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer ws.Close()

	// 2. 受け取ったリクエストをクライアントとして登録
	clients[ws] = true

	// 3. websocketからのメッセージを待機
	for {
		var msg Message
		err := ws.ReadJSON(&msg)
		if err != nil {
			log.Printf("error: %v", err)
			delete(clients, ws)
			break
		}
		// 4. メッセージを受信するとbroadcastチャネルに送信
		// <- は ”brodacastチャンネルにmsgを送る”という意味
		broadcast <- msg
	}
}

// broadcastチャネルにメッセージが送信された時、それを取り出す役
// -> handleConnectionsでハンドリングしているクライアントのどれかがメッセージを送信すると、 そのメッセージを取り出して現在接続しているクライアント全てにそのメッセージを送信
func handleMessages() {
	for {
		msg := <-broadcast
		for client := range clients {
			// ブロードキャスト
			err := client.WriteJSON(msg)
			if err != nil {
				log.Printf("error: %v", err)
				client.Close()
				delete(clients, client)
			}
		}
	}
}
