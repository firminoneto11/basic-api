package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/valyala/fasthttp"
)

type Controller struct {
	eventsChannel chan Event
}

type Event struct {
	Type string `json:"type"`
	Data string `json:"data"`
}

func main() {
	app := getApp()
	eventsChannel := make(chan Event, 1_000)
	setCors(app)
	addRouter(app, eventsChannel)
	app.Listen(":5555")
}

func getApp() *fiber.App {
	app := fiber.New()
	return app
}

func addRouter(app *fiber.App, eventsChannel chan Event) {
	router := app.Group("/api")

	controller := Controller{eventsChannel}

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

func (instance *Controller) sse(ctx *fiber.Ctx) error {
	// Upgrading the HTTP connection to be in the SSE format
	ctx.Set("Content-Type", "text/event-stream")
	ctx.Set("Cache-Control", "no-cache")
	ctx.Set("Connection", "keep-alive")
	ctx.Set("Transfer-Encoding", "chunked")

	streamWriter := fasthttp.StreamWriter(
		func(ioWriter *bufio.Writer) {

			go func() {
				for {
					fmt.Fprintf(ioWriter, "event: heartbeat\n\n")
					err := ioWriter.Flush()
					if err != nil {
						break
					}
					time.Sleep(time.Second)
				}
			}()

			for event := range instance.eventsChannel {
				binary, err := json.Marshal(event)
				if err != nil {
					fmt.Println(err)
					continue
				}

				message := "event: message\ndata: " + string(binary) + "\n\n"
				fmt.Fprint(ioWriter, message)

				err = ioWriter.Flush()
				if err != nil {
					break
				}
			}
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

	instance.eventsChannel <- event

	return ctx.JSON(fiber.Map{"details": "Message sent successfully!"})
}
