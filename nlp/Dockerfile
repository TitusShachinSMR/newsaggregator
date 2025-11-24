# ---------- BUILD STAGE ----------
FROM golang:1.23-alpine AS build

# Set working directory
WORKDIR /src

# Copy go.mod and go.sum first (for caching)
COPY go.mod go.sum ./

# Enable Go proxy inside Docker (recommended)
RUN go env -w GOPROXY=https://proxy.golang.org,direct \
    && go env -w GOSUMDB=sum.golang.org

# Download dependencies
RUN go mod download

# Copy rest of the source
COPY . .

# Build binary
RUN go build -o /app/nlp .

# ---------- RUNTIME STAGE ----------
FROM alpine:3.18

RUN apk add --no-cache ca-certificates

# Copy binary from build stage
COPY --from=build /app/nlp /usr/local/bin/nlp

# Default command
CMD ["nlp"]
