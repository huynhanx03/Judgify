FROM ubuntu:22.04

# Avoid interactive prompts during package installation
ENV DEBIAN_FRONTEND=noninteractive

# Install compilers and runtimes: C++, Python, Java, Go
RUN apt-get update && apt-get install -y --no-install-recommends \
    g++ \
    python3 \
    default-jdk-headless \
    golang-go \
    && rm -rf /var/lib/apt/lists/*

# Create sandbox working directory
RUN mkdir -p /sandbox && chmod 777 /sandbox
WORKDIR /sandbox

# Default command: keep container alive for docker exec
CMD ["sleep", "infinity"]
