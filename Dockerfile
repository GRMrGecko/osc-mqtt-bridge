FROM golang:1.20

# Build app
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY *.go ./
RUN go build -o /osc-mqtt-bridge
WORKDIR /app
RUN rm -Rf /app; mkdir /etc/osc-mqtt-bridge

# Configuration volume
VOLUME ["/etc/osc-mqtt-bridge"]

# Command
CMD ["/osc-mqtt-bridge"]
