FROM node:22-alpine AS frontend
WORKDIR /app/frontend
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

FROM golang:1.25-alpine AS backend
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /app/frontend/dist ./frontend/dist
RUN go build -o /cinema-server .

FROM alpine:3.21
RUN addgroup -S cinema && adduser -S cinema -G cinema
WORKDIR /app
COPY --from=backend /cinema-server /app/cinema-server
COPY static /app/static
COPY --from=backend /src/frontend/dist /app/frontend/dist
USER cinema
EXPOSE 8080
CMD ["./cinema-server"]
