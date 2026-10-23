# Stage 1: Build binary
FROM golang:1.26-alpine AS builder
WORKDIR /src

COPY discovery ./discovery
COPY mensajeria ./mensajeria
COPY auditoria ./auditoria
WORKDIR /src/auditoria
RUN CGO_ENABLED=0 GOOS=linux go build -o main .

# Stage 2: Final lightweight image
FROM alpine:3.19
WORKDIR /app
COPY --from=builder /src/auditoria/main .

EXPOSE 8080
CMD ["./main"]
