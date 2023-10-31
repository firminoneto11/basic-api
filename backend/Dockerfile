FROM golang:1.20-alpine AS build

WORKDIR /app

RUN cd /app

COPY go.mod .

COPY go.sum .

RUN go mod download

RUN go mod tidy

COPY . .

RUN go build -o /app/bin/app .

RUN chmod +x /app/bin/app

FROM alpine:latest

COPY --from=build /app/bin/app /usr/bin/app

EXPOSE 5555

CMD ["app"]
