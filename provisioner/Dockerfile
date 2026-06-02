FROM golang:1.22-alpine AS build

WORKDIR /src
COPY go.mod .
RUN go mod download
COPY . .
RUN go build -o /out/provisioner ./main.go

FROM alpine:3.20
RUN apk add --no-cache ca-certificates docker-cli docker-cli-compose
WORKDIR /app
COPY --from=build /out/provisioner /app/provisioner
EXPOSE 9002
ENTRYPOINT ["/app/provisioner"]
