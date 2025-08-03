# mdns-publisher

A Go application that automatically publishes Docker container hostnames to mDNS (multicast DNS) for local network discovery.

The intention is to be used as a sidecar to a Traefik instance.

## What it does

- Monitors Docker containers for Traefik router labels
- Extracts hostnames from `traefik.http.routers.*.rule` labels
- Publishes these hostnames via mDNS with a specified IP address
- Automatically updates when containers start/stop

## Usage

```bash
# Build the binary
make

# Run with default settings (5s polling interval)
./mdns-publisher

# Specify burst gate interval (prevents excessive updates during rapid container changes)
./mdns-publisher -interval=10s

# Specify IP address to publish
./mdns-publisher -ip=192.168.1.100

# Use HOSTNAME_IP environment variable
HOSTNAME_IP=192.168.1.100 ./mdns-publisher
```

## Requirements

- Docker daemon running
- Containers with Traefik labels in format: `traefik.http.routers.{name}.rule=Host(`{hostname}`)`

## Development

```bash
# Run all checks
make check

# Build and run
make && ./mdns-publisher
``` 

## History behind this

Originally, I wrote a simple script to handle dns updates for my own home server: https://mortz.dev/posts/2025/06/docker-mdns-publisher/

Then I figured, why not make it a bigger project? and this is the result.

### What's next?

I'm thinking about making this project more generic, so it can be used by any other application (not just Traefik), and publish any kind of DNS records (not just A records), to any DNS server (e.g. Cloudflare, or even CoreDNS)!

I haven't decided yet on how it's gonna look like, but following the footsteps of similar projects (where container lables are used for configuration of third party services), I'm thinking about using labels too.