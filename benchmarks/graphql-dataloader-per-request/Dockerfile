FROM node:22-slim AS builder

RUN apt-get update && apt-get install -y python3 make g++ && rm -rf /var/lib/apt/lists/* && \
    npm config set update-notifier false

WORKDIR /build
COPY app/package.json app/tsconfig.json ./
RUN npm install --legacy-peer-deps

COPY app/src/ ./src/
RUN npx tsc

FROM node:22-slim

RUN apt-get update && \
    apt-get install -y bash curl git python3 make g++ && \
    rm -rf /var/lib/apt/lists/* && \
    git config --global user.email "bench@needle-bench.cc" && \
    git config --global user.name "needle-bench"

WORKDIR /app

COPY app/package.json app/tsconfig.json ./
RUN npm install --legacy-peer-deps

COPY --from=builder /build/dist/ ./dist/
COPY app/src/ ./src/

# Seed the database
RUN mkdir -p /app/data && node dist/seed.js

COPY test.sh /app/test.sh
RUN chmod +x /app/test.sh

EXPOSE 4000

CMD ["node", "dist/server.js"]
