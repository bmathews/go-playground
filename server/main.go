package main

import (
	"encoding/json"
	"flag"
	"github.com/garyburd/redigo/redis"
	"github.com/googollee/go-socket.io"
	"log"
	"net/http"
	"strings"
	"time"
)

type Message struct {
	User      string    `json:"user"`
	Text      string    `json:"text"`
	Timestamp time.Time `json:"timestamp"`
}

// Create a redis connection pool
func newPool(server, password string) *redis.Pool {
	return &redis.Pool{
		MaxIdle:     3,
		IdleTimeout: 240 * time.Second,
		Dial: func() (redis.Conn, error) {
			log.Println("Dialing redis")
			c, err := redis.Dial("tcp", server)
			if err != nil {
				return nil, err
			}

			return c, err
		},
		TestOnBorrow: func(c redis.Conn, t time.Time) error {
			_, err := c.Do("PING")
			return err
		},
	}
}

// Handle a socket chat message
func handleMessage(so socketio.Socket, pool redis.Pool, msgs chan Message, msg Message) {
	jsonString, _ := json.Marshal(msg)

	c := pool.Get()
	defer c.Close()

	so.Emit("chat message", jsonString)                 // send to self
	so.BroadcastTo("chat", "chat message", jsonString)  // send to others
	c.Do("ZADD", "chat", time.Now().Unix(), jsonString) // add to redis
	msgs <- msg                                         // send to msg channel for pubsub
}

// Handle a socket connection
func handleConnection(so socketio.Socket, pool redis.Pool, msgs chan Message) {
	log.Printf("New connection")

	c := pool.Get()
	defer c.Close()

	so.Join("chat")

	val, _ := time.ParseDuration("100h")
	past := time.Now().Add(-val).Unix()

	res, err := redis.Strings(c.Do("ZRANGEBYSCORE", "chat", past, time.Now().Unix()))
	perror(err)

	log.Printf("Sending chat history %s", strings.Join(res, "\n"))
	so.Emit("chat history", strings.Join(res, "\n"))
	so.On("chat message", func(jsonMsg string) {
		log.Printf("Got new message, deserializing: %s", jsonMsg)
		// Parse text as json and decode into message struct
		var message Message
		err := json.NewDecoder(strings.NewReader(jsonMsg)).Decode(&message)
		perror(err)
		handleMessage(so, pool, msgs, message)
	})
}

// Blocking subscription routine
func runSubs(pool redis.Pool, server socketio.Server) {
	c := pool.Get()
	defer c.Close()

	psc := redis.PubSubConn{c}
	psc.Subscribe("chat")
	for {
		switch v := psc.Receive().(type) {
		case redis.Message:
			log.Printf("%s: message: %s\n", v.Channel, v.Data)
			server.BroadcastTo("chat", "chat message", string(v.Data[:]))
		case error:
			perror(v)
		}
	}
}

// Blocking publish routine for channel
func runPubs(pool redis.Pool, msgs chan Message) {
	c := pool.Get()
	defer c.Close()

	// Forever iterate through msgs channel
	for msg := range msgs {
		jsonString, err := json.Marshal(msg)
		perror(err)
		log.Printf("Got message from channel, publishing: %s", jsonString)
		c.Do("PUBLISH", "chat", jsonString)
	}
}

var (
	pool          *redis.Pool
	httpPort      = flag.String("port", "5000", "")
	id            = flag.String("id", "id", "")
	redisServer   = flag.String("redisServer", ":6379", "")
	redisPassword = flag.String("redisPassword", "", "")
)

func main() {
	// Parse cli flags
	flag.Parse()

	// Create connection pool
	pool = newPool(*redisServer, *redisPassword)

	// Create socketio server
	server, err := socketio.NewServer(nil)
	perror(err)

	// Make msgs channel
	msgs := make(chan Message)

	// Start routines
	go runSubs(*pool, *server)
	go runPubs(*pool, msgs)

	// Listen for socket connections
	server.On("connection", func(so socketio.Socket) {
		handleConnection(so, *pool, msgs)
	})

	// Listen for socket errors
	server.On("error", func(so socketio.Socket, err error) {
		log.Println("error:", err)
	})

	// Serve socketio server at path
	http.Handle("/socket.io/", server)

	// Serve static files
	http.Handle("/", http.FileServer(http.Dir("./client")))

	// Listen and serve
	log.Printf("Server %s up at localhost:%s...", *id, *httpPort)
	log.Fatal(http.ListenAndServe(":"+*httpPort, nil))
}

func perror(err error) {
	if err != nil {
		panic(err)
	}
}
