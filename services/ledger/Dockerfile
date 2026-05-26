FROM golang:1.25.10-alpine AS base
WORKDIR /app
ENV GOMAXPROCS=2

FROM base AS deps
COPY go.mod go.sum ./
RUN go mod download

FROM deps AS dev
ENV APP_ENV=development
RUN go install github.com/air-verse/air@v1.65.1
COPY . .
EXPOSE 4003
CMD ["air", "-c", ".air.toml"]

FROM deps AS build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -tags nomsgpack -p 2 -o /out/main ./cmd/api

FROM alpine:3.20 AS prod
WORKDIR /app
RUN apk add --no-cache ca-certificates \
  && addgroup -S app \
  && adduser -S -G app app
COPY --from=build /out/main ./main
COPY --from=build /app/migrations ./migrations
EXPOSE 4003
USER app
CMD ["./main"]

FROM dev AS default
