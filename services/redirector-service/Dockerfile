# --- Stage 1: Build Stage ---
# Starting with a Go-optimized image for compilation
FROM golang:1.26-alpine AS builder

# We'll receive the application name to build as an argument during build time
ARG APP_NAME

# Creating a working directory for application source code
WORKDIR /app

# First, copy only the module files and download dependencies
# This layer only runs when dependencies change, speeding up builds
COPY pkg/go.mod pkg/go.sum ./pkg/
COPY services/${APP_NAME}-service/go.mod services/${APP_NAME}-service/go.sum ./services/${APP_NAME}-service/
RUN cd services/${APP_NAME}-service && go mod tidy && go mod download

# Copy all source code
COPY . .

# Build our application
# CGO_ENABLED=0 creates a static binary for seamless operation in minimal images
# Save the output to /app/bin directory with the application name from the argument
RUN CGO_ENABLED=0 go build -o /app/bin/${APP_NAME} ./services/${APP_NAME}-service/cmd

# --- Stage 2: Final Stage ---
# An ultra-minimal alpine image containing only what's needed to run the application
FROM alpine:latest

# Get security updates
RUN apk --no-cache add ca-certificates

# Copy our compiled application from the 'builder' stage into this image
COPY --from=builder /app/bin/* /usr/local/bin/

# Specify which command will run when the container starts
# The command from docker-compose will override this, but it's good to have a default
CMD ["/usr/local/bin/redirector"]
