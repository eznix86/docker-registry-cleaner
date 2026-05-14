FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /drc ./cmd/drc

FROM alpine:3.23

RUN apk --no-cache add ca-certificates

COPY --from=builder /drc /usr/local/bin/drc

ENTRYPOINT ["/usr/local/bin/drc"]
