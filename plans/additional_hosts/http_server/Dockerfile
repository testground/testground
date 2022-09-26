FROM golang:1.19.1-alpine

WORKDIR /app

COPY go.mod ./

COPY *.go ./

RUN go build -o /http_server
EXPOSE 8080

CMD [ "/http_server" ]
