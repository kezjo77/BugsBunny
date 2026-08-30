# --- Build stage ---
FROM golang:1.25-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o bugsbunny .

# --- Run stage ---
FROM alpine:3.20
WORKDIR /app
RUN mkdir -p /app/uploads
COPY --from=build /app/bugsbunny ./bugsbunny
COPY --from=build /app/plugins ./plugins
COPY --from=build /app/schema.sql ./schema.sql
COPY --from=build /app/seed.sql ./seed.sql

# Render/Railway inject PORT at runtime; the app reads it via os.Getenv("PORT")
EXPOSE 8080
CMD ["./bugsbunny"]
