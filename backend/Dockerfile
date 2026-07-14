FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /quiz-backend ./main.go

FROM alpine:3.20

WORKDIR /

COPY --from=builder /quiz-backend /quiz-backend

COPY .env .env

EXPOSE 8080

CMD ["/quiz-backend"]