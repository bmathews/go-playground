package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/garyburd/redigo/redis"
	"github.com/googollee/go-socket.io"
	"github.com/jmoiron/jsonq"
	"github.com/parnurzeal/gorequest"
)

type message struct {
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
func handleMessage(so socketio.Socket, pool redis.Pool, msgs chan message, msg message) {
	jsonString, _ := json.Marshal(msg)

	c := pool.Get()
	defer c.Close()

	so.Emit("chat message", jsonString)                           // send to self
	so.BroadcastTo("chat", "chat message", jsonString)            // send to others
	_, err := c.Do("ZADD", "chat", time.Now().Unix(), jsonString) // add to redis
	perror(err)

	msgs <- msg // send to msg channel for pubsub

	// If the message starts with /wiki, hit the wiki search api with that query and send result
	if strings.HasPrefix(msg.Text, "/wiki ") {
		query := msg.Text[5:]
		request := gorequest.New()
		_, body, _ := request.Get(fmt.Sprintf("https://en.wikipedia.org/w/api.php?action=opensearch&search=%s&limit=2&namespace=0&format=json", query)).End()

		// Wrap json response in an object since it comes back as a flat array
		body = "{ \"data\": " + body + " }"

		// Decode into map of string->anything
		data := map[string]interface{}{}
		json.NewDecoder(strings.NewReader(body)).Decode(&data)

		// Query with jsonq
		jq := jsonq.NewQuery(data)
		match, _ := jq.String("data", "1", "0")
		summary, _ := jq.String("data", "2", "0")
		link, _ := jq.String("data", "3", "0")

		// Create a new message
		m := message{User: "Bot", Text: fmt.Sprintf("<a href=\"%s\">%s</a>: %s", link, match, summary), Timestamp: time.Now()}

		// Send
		handleMessage(so, pool, msgs, m)
	}
}

// Handle a socket connection
func handleConnection(so socketio.Socket, pool redis.Pool, msgs chan message) {
	log.Printf("New connection")

	c := pool.Get()
	defer c.Close()

	// Join "chat" socket channel
	so.Join("chat")

	// Fetch chat history from the past 100 hours
	val, _ := time.ParseDuration("100h")
	past := time.Now().Add(-val).Unix()
	res, err := redis.Strings(c.Do("ZRANGEBYSCORE", "chat", past, time.Now().Unix()))
	perror(err)

	// Send chat history to socket
	log.Printf("Sending chat history %s", strings.Join(res, "\n"))
	so.Emit("chat history", strings.Join(res, "\n"))

	// Listen for chat messages
	so.On("chat message", func(jsonMsg string) {
		log.Printf("Got new message, deserializing: %s", jsonMsg)
		// Parse text as json and decode into message struct
		var message message
		err := json.NewDecoder(strings.NewReader(jsonMsg)).Decode(&message)
		perror(err)
		handleMessage(so, pool, msgs, message)
	})
}

// Blocking subscription routine
func runSubs(pool redis.Pool, server socketio.Server) {
	c := pool.Get()
	defer c.Close()

	// Subscribe to redis pubsub channel "chat"
	psc := redis.PubSubConn{Conn: c}
	psc.Subscribe("chat")

	// Forever switch on the eventual values of psc.Receive()
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
func runPubs(pool redis.Pool, msgs chan message) {
	// Forever iterate through eventual values in the msgs channel
	for msg := range msgs {
		c := pool.Get()
		jsonString, err := json.Marshal(msg)
		perror(err)
		log.Printf("Got message from channel, publishing: %s", jsonString)
		_, err2 := c.Do("PUBLISH", "chat", jsonString) // publish to redis chat channel
		perror(err2)
		c.Close()
	}
}

func logWrapper(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		dir, _ := filepath.Abs(filepath.Dir(os.Args[0]))
		if _, err := os.Stat(dir + "/public" + r.URL.Path); err == nil {
			log.Println("200: ", dir)
		} else {
			log.Println("404: ", dir)
		}
		h.ServeHTTP(w, r)
	})
}

var (
	pool          *redis.Pool
	httpPort      = flag.String("port", "8080", "")
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
	msgs := make(chan message)

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

	http.Handle("/status", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "ok")
	}))

	// Serve static files
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	http.Handle("/", logWrapper(http.FileServer(http.Dir(dir+"/public"))))

	// Listen and serve
	log.Printf("Server %s up at localhost:%s...", *id, *httpPort)
	log.Fatal(http.ListenAndServe(":"+*httpPort, logWrapper(http.DefaultServeMux)))
}

func perror(err error) {
	if err != nil {
		panic(err)
	}
}
