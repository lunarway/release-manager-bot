FROM golang:1.20.0 as builder
WORKDIR /app
ENV CGO_ENABLED=0
ENV GOOS=linux
ENV GOARCH=amd64

COPY go.mod .
COPY go.sum .
RUN go mod download

COPY main.go .

RUN go build -o release-manager-bot main.go

FROM scratch
WORKDIR /app

COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /app/release-manager-bot .

ENTRYPOINT [ "./release-manager-bot" ]
