package main

import (
	"log"
	"time"
)

// send three messages to the channel with a 1 second wait inbetween
func doStuffAsync(msgs chan string) {
	msgs <- "async message one"
	time.Sleep(1 * time.Second)
	msgs <- "async message two"
	time.Sleep(1 * time.Second)
	msgs <- "async message three"
}

func logStuff(msgs chan string) {
	// for every message that gets sent to this channel, perform a log
	for msg := range msgs {
		log.Printf(msg)
	}
}

func main() {
	msgs := make(chan string)

	go doStuffAsync(msgs) // spawns a new thread
	go logStuff(msgs)     // spawns a new thread

	time.Sleep(3 * time.Second) // wait for everything to finish

	log.Printf("Finished!")
}
