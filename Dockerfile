FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY migrations/ ./migrations/
FROM build AS migration-build
RUN CGO_ENABLED=0 go build -o /out/anime-migrate ./cmd/migrate
FROM alpine:3.21 AS migrator
RUN adduser -D -H migrate
COPY --from=migration-build /out/anime-migrate /usr/local/bin/anime-migrate
USER migrate
ENTRYPOINT ["/usr/local/bin/anime-migrate"]

FROM build AS api-build
RUN CGO_ENABLED=0 go build -o /out/anime-api ./cmd/api
FROM alpine:3.21 AS api
RUN adduser -D -H app
WORKDIR /app
COPY --from=api-build /out/anime-api /app/anime-api
RUN mkdir -p /app/data && chown -R app:app /app
USER app
EXPOSE 8080
ENTRYPOINT ["/app/anime-api"]
