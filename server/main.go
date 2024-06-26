package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/streadway/amqp"
)

var (
	upgrader = websocket.Upgrader{}
	clients  = make(map[*websocket.Conn]bool)
	broker   = "amqp://localhost:5672"
	queue    = "data-stream"
)

func main() {
	go consumeMessages()

	http.HandleFunc("/websocket", handleWebSocket)
	log.Println("Server started on :8888")
	log.Fatal(http.ListenAndServe(":8888", nil))
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	defer conn.Close()

	clients[conn] = true

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			log.Println(err)
			delete(clients, conn)
			break
		}

		go writeToRabbitMQ(string(message))
	}
}

func writeToRabbitMQ(message string) {
	conn, err := amqp.Dial(broker)
	if err != nil {
		log.Println(err)
		return
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		log.Println(err)
		return
	}
	defer ch.Close()

	q, err := ch.QueueDeclare(
		queue,
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		log.Println(err)
		return
	}

	err = ch.Publish(
		"",
		q.Name,
		false,
		false,
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        []byte(message),
		})
	if err != nil {
		log.Println(err)
		return
	}

	fmt.Printf("Message sent to RabbitMQ: %s\n", message)
}

func consumeMessages() {
	conn, err := amqp.Dial(broker)
	if err != nil {
		log.Println(err)
		return
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		log.Println(err)
		return
	}
	defer ch.Close()

	q, err := ch.QueueDeclare(
		queue,
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		log.Println(err)
		return
	}

	msgs, err := ch.Consume(
		q.Name,
		"",
		true,
		false,
		false,
		false,
		nil,  
	)
	if err != nil {
		log.Println(err)
		return
	}

	for msg := range msgs {
		for client := range clients {
			err := client.WriteMessage(websocket.TextMessage, msg.Body)
			if err != nil {
				log.Println(err)
				client.Close()
				delete(clients, client)
			}
		}
	}
}
