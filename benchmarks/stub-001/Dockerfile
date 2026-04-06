FROM node:20-alpine

# Benchmark generated from facebook/react @ 1b45e2439289
# Language: javascript

RUN apk add --no-cache bash curl git 2>/dev/null || \
    apt-get update && apt-get install -y bash curl git

RUN git config --global user.email "bench@needle-bench.cc" && \
    git config --global user.name "needle-bench"

WORKDIR /workspace

# TODO: Replace with actual repo checkout + build steps.
#   git clone --depth=1 --branch <branch> https://github.com/facebook/react.git .
#   git checkout 1b45e2439289
COPY app/ /workspace/app/
COPY test.sh /workspace/test.sh
RUN chmod +x test.sh

CMD ["bash"]
