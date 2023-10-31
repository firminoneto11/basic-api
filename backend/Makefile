dev:
	go run main.go

build:
	docker build -t backend .
	docker run --rm -p 5555:5555 --name backend-golang backend
