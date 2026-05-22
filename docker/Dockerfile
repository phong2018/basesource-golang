FROM golang:1.23 AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -o app .

FROM gcr.io/distroless/static-debian11:latest AS final
COPY --from=builder /src/app /usr/local/bin/app
COPY --from=builder /src/db/migrations /app/db/migrations
WORKDIR /
ENTRYPOINT ["/usr/local/bin/app"]
CMD ["api"]
