# syntax=docker/dockerfile:1

FROM node:24-bookworm AS build

WORKDIR /app
COPY package.json package-lock.json ./
RUN --mount=type=cache,target=/root/.npm npm install -g npm@latest && \
    npm ci
COPY . .
ARG NEXT_PUBLIC_VAPID_PUBLIC_KEY
ENV NEXT_PUBLIC_VAPID_PUBLIC_KEY=$NEXT_PUBLIC_VAPID_PUBLIC_KEY
ENV NEXT_TELEMETRY_DISABLED=1
RUN --mount=type=cache,target=/root/.npm \
    --mount=type=cache,target=/app/.next/cache \
    npm run build

FROM build AS test

FROM build AS lint

CMD ["sh", "-c", "npm run lint:prettier && npm run lint && npm run knip"]

FROM node:24-bookworm AS dev

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
    echo "${APP_USER}" > /tmp/appuser

WORKDIR /app
COPY package.json package-lock.json ./
RUN --mount=type=cache,target=/root/.npm npm install -g npm@latest && \
    npm ci

COPY docker-entrypoint.sh /docker-entrypoint.sh
RUN chmod +x /docker-entrypoint.sh

ENV NEXT_TELEMETRY_DISABLED=1

EXPOSE 3000

ENTRYPOINT ["/docker-entrypoint.sh"]
CMD ["npm", "run", "dev"]

FROM node:24-bookworm AS run

WORKDIR /app
ENV NODE_ENV=production
ENV NEXT_TELEMETRY_DISABLED=1

COPY --from=build /app/node_modules ./node_modules
COPY --from=build /app/.next ./.next
COPY --from=build /app/public ./public
COPY --from=build /app/package.json ./

USER node

EXPOSE 3000

CMD ["npm", "start"]
