package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/google/uuid"
	"github.com/valyala/fasthttp"
)

func main() {
	app := fiber.New()
	setCors(app)
	addRouter(app)
	app.Listen(":5555")
}

func addRouter(app *fiber.App) {
	router := app.Group("/api")

	messageBus := make([]*Channel, 0)
	controller := Controller{messageBus: messageBus}

	router.Get("/sse/", controller.sse)
	router.Post("/events/", controller.newEvent)
}

func setCors(app *fiber.App) {
	config := cors.Config{
		AllowOrigins: "*",
		AllowHeaders: "*",
	}
	app.Use(cors.New(config))
}

func uuid4() string {
	id := uuid.New()
	return id.String()
}

func sendHeartbeat(iow *bufio.Writer, ctr *Controller, channel *Channel) {
	for {
		message := "event: message\ndata: " + `{"type": "heartbeat", "data": null}` + "\n\n"
		fmt.Fprint(iow, message)

		err := iow.Flush()
		if err != nil {
			ctr.remove(channel)
			break
		}

		time.Sleep(time.Second)
	}
}

func listenChannel(iow *bufio.Writer, ctr *Controller, channel *Channel) {
	for event := range channel.ch {
		byteArray, err := json.Marshal(event)
		if err != nil {
			fmt.Println(err)
			continue
		}

		message := "event: message\ndata: " + string(byteArray) + "\n\n"
		fmt.Fprint(iow, message)

		err = iow.Flush()
		if err != nil {
			ctr.remove(channel)
			break
		}
	}
}

type Event struct {
	Type string `json:"type"`
	Data string `json:"data"`
}

type Channel struct {
	id     string
	closed bool
	ch     chan Event
	mutex  sync.RWMutex
}

func (instance *Channel) close() {
	instance.mutex.Lock()
	defer instance.mutex.Unlock()
	if instance.closed {
		return
	}
	close(instance.ch)
	instance.closed = true
}

type Controller struct {
	messageBus []*Channel
}

func (instance *Controller) sse(ctx *fiber.Ctx) error {
	// Upgrading the HTTP connection to be in the SSE format
	ctx.Set("Content-Type", "text/event-stream")
	ctx.Set("Cache-Control", "no-cache")
	ctx.Set("Connection", "keep-alive")
	ctx.Set("Transfer-Encoding", "chunked")

	eventsChannel := &Channel{id: uuid4(), ch: make(chan Event, 1_000), closed: false}
	instance.add(eventsChannel)

	streamWriter := fasthttp.StreamWriter(
		func(ioWriter *bufio.Writer) {
			go sendHeartbeat(ioWriter, instance, eventsChannel)
			listenChannel(ioWriter, instance, eventsChannel)
		},
	)

	// Starts streaming inside this goroutine
	ctx.Context().SetBodyStreamWriter(streamWriter)

	return nil
}

func (instance *Controller) newEvent(ctx *fiber.Ctx) error {
	event := Event{}
	err := json.Unmarshal(ctx.Body(), &event)
	if err != nil || event.Data == "" {
		fmt.Println(err)
		ctx.SendStatus(400)
		return ctx.JSON(fiber.Map{"details": "Invalid JSON format"})
	}

	if event.Type == "" {
		event.Type = "default"
	}

	instance.broadcast(event)

	return ctx.JSON(fiber.Map{"details": "Message sent successfully!"})
}

func (instance *Controller) broadcast(event Event) {
	if len(instance.messageBus) == 0 {
		return
	}
	for _, channel := range instance.messageBus {
		channel.mutex.RLock()
		if !channel.closed {
			channel.ch <- event
		}
		channel.mutex.RUnlock()
	}
}

func (instance *Controller) add(channel *Channel) {
	instance.messageBus = append(instance.messageBus, channel)
}

func (instance *Controller) remove(channel *Channel) {
	newArray := make([]*Channel, 0)
	for _, value := range instance.messageBus {
		if value.id != channel.id {
			newArray = append(newArray, value)
		}
	}
	instance.messageBus = newArray
	channel.close()
}
