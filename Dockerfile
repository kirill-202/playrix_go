# Build stage
FROM golang:1.23-alpine AS builder

# Set the working directory
WORKDIR /app

# Install Git to fetch the code from the repository
RUN apk add --no-cache git

# Clone the repository from an argument
ARG REPO_URL
ARG BRANCH
RUN git clone -b $BRANCH $REPO_URL .

# Download dependencies and build the Go binary
RUN go mod download
RUN go build -o myapp .

# Runtime stage
FROM alpine:3.18

# Accept runtime environment variables as arguments
ARG PLAYRIX_SPREAD_SHEET_ID
ARG GOOGLE_CRED_PATH
ARG GREEDLY_API_KEY
ARG GREEDLY_DATABASE_ID
ARG SHEET_NAMES

ENV PLAYRIX_SPREAD_SHEET_ID=$PLAYRIX_SPREAD_SHEET_ID \
    GOOGLE_CRED_PATH=$GOOGLE_CRED_PATH \
    GREEDLY_API_KEY=$GREEDLY_API_KEY \
    GREEDLY_DATABASE_ID=$GREEDLY_DATABASE_ID \
    SHEET_NAMES=$SHEET_NAMES

# Set the working directory and copy the binary
WORKDIR /app
COPY --from=builder /app/myapp .

# Mount the client_secret.json file
VOLUME [ "/app/client_secret.json" ]

CMD ["./myapp"]
