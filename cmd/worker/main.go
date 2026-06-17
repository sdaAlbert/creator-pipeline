package main

import "log"

func main() {
	log.Println("The local demo starts an embedded worker in cmd/api.")
	log.Println("Replace internal/queue with RabbitMQ and run this process as an independent consumer in the distributed version.")
}
