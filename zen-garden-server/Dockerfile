FROM golang:alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o zen-garden-server .

FROM alpine:latest
WORKDIR /app
COPY --from=build /app/zen-garden-server .
EXPOSE 8080
CMD ["./zen-garden-server"]
