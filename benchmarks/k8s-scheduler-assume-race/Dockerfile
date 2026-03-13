FROM golang:1.24

RUN apt-get update && apt-get install -y --no-install-recommends bash && rm -rf /var/lib/apt/lists/*

WORKDIR /workspace
RUN git config --global user.email "bench@needle-bench.cc" && git config --global user.name "bench"
# Minimal reproduction of the k8s scheduler assume-race bug
RUN mkdir -p pkg/scheduler

COPY app/ /workspace/

# The test verifies the fix
COPY test.sh /workspace/test.sh
RUN chmod +x /workspace/test.sh
