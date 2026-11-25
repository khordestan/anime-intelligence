FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY migrations/ ./migrations/
RUN CGO_ENABLED=0 go build -o /out/anime-api ./cmd/api
FROM alpine:3.21
RUN adduser -D -H app
WORKDIR /app
COPY --from=build /out/anime-api /app/anime-api
RUN mkdir -p /app/data && chown -R app:app /app
USER app
EXPOSE 8080
ENTRYPOINT ["/app/anime-api"]
