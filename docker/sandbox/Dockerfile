FROM alpine:3.20

# Install compilers and runtimes: C++, Python, Java, Go
RUN apk add --no-cache \
    g++ \
    python3 \
    openjdk17-jdk \
    go

# Create sandbox working directory
RUN mkdir -p /sandbox && chmod 777 /sandbox
WORKDIR /sandbox

# Default command: keep container alive for docker exec
CMD ["sleep", "infinity"]
