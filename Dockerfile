FROM golang:1.23.3 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download
COPY ./cmd/collect ./cmd/collect
RUN CGO_ENABLED=0 GOOS=linux go build -o /collect ./cmd/collect/main.go

FROM gcr.io/distroless/static:nonroot

COPY --from=builder /collect /

USER nonroot:nonroot

ENTRYPOINT ["/collect"]
