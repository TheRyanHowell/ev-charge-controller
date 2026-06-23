# syntax=docker/dockerfile:1

FROM node:24-bookworm AS base

WORKDIR /app

# System dependencies for Playwright browsers (Chromium + Firefox)
RUN apt-get update && apt-get install -y --no-install-recommends \
    libnss3 \
    libatk1.0-0 \
    libatk-bridge2.0-0 \
    libcups2 \
    libxcomposite1 \
    libxrandr2 \
    libxdamage1 \
    libgbm1 \
    libpango-1.0-0 \
    libcairo2 \
    libasound2 \
    libxshmfence1 \
    libdrm2 \
    libx11-xcb1 \
    libxcb1 \
    fonts-liberation \
    libappindicator3-1 \
    libxtst6 \
    xdg-utils \
    wget \
    && rm -rf /var/lib/apt/lists/*

COPY package.json package-lock.json ./
RUN --mount=type=cache,target=/root/.npm npm install -g npm@latest && \
    npm ci

# Install Playwright browsers with system deps
RUN npx playwright install-deps chromium firefox && \
    npx playwright install chromium firefox

COPY . .

FROM base AS dev

ARG UID=1000
ARG GID=1000

# node image ships with user `node` at 1000:1000 - reuse if host matches, else create new
RUN apt-get update && apt-get install -y --no-install-recommends gosu && rm -rf /var/lib/apt/lists/* && \
    if [ "${UID}" = "0" ]; then \
      APP_USER=root; \
    elif [ "${UID}" = "1000" ] && [ "${GID}" = "1000" ]; then \
      usermod -l appuser node && groupmod -n appuser node && \
      APP_USER=appuser; \
    else \
      groupadd -g "${GID}" appuser && useradd -u "${UID}" -g "${GID}" -m appuser && \
      APP_USER=appuser; \
    fi && \
    echo "${APP_USER}" > /tmp/appuser && \
    chown -R "${APP_USER}:${APP_USER}" /app && \
    # Move Playwright browsers to shared location (accessible by non-root user)
    mkdir -p /opt/ms-playwright && \
    cp -r /root/.cache/ms-playwright/* /opt/ms-playwright/ && \
    chmod -R a+rX /opt/ms-playwright

# Point Playwright to the browser cache in shared location
ENV PLAYWRIGHT_BROWSERS_PATH=/opt/ms-playwright

COPY docker-entrypoint.sh /docker-entrypoint.sh
RUN chmod +x /docker-entrypoint.sh

ENTRYPOINT ["/docker-entrypoint.sh"]
CMD ["npx", "playwright", "test"]

FROM base AS lint

CMD ["sh", "-c", "npx prettier --check \"**/*.ts\" && npx eslint \"tests/**/*.ts\" && npx tsc --noEmit"]

FROM base AS ci

CMD ["npx", "playwright", "test", "--reporter=list"]
