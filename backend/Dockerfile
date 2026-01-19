# Use golang with air for development
FROM golang:1.25.6-alpine3.23

# Set the working directory
WORKDIR /app

# Copy the source code into the container
COPY . .

# Install the dependencies
RUN go mod download

# Install the air CLI
RUN go install github.com/air-verse/air@latest

# Expose the port
EXPOSE 1323

# Run the application
CMD ["air"  , "-c", ".air.toml"]
