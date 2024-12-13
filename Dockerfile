# Use the golang image as the builder
FROM golang:1.22.3-alpine AS builder

# Set the working directory
WORKDIR /app

# Copy the source code into the container
COPY . .

# Download go modules
RUN go mod download

# Tidy up the go modules
RUN go mod tidy

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-extldflags "-static"' -o rencana-kerja-cron cmd/rencana-kerja/main.go \
  && CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-extldflags "-static"' -o data-karyawan-cron cmd/data-karyawan/main.go

# Build a smaller image that will only contain the application's binary
FROM alpine:latest

# Set the working directory
WORKDIR /app

# Copy the application's binary
COPY --from=builder /app/rencana-kerja/main ./cmd/rencana-kerja/main \
&& --from=builder /app/data-karyawan/main ./cmd/data-karyawan/main

# Ensure the binary has execute permissions
RUN chmod +x ./cmd/rencana-kerja/main \ 
&& chmod +x ./cmd/data-karyawan/main \