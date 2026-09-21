# syntax=docker/dockerfile:1.7
ARG NODE_BUILDER_IMAGE
ARG NGINX_RUNTIME_IMAGE
FROM --platform=$BUILDPLATFORM ${NODE_BUILDER_IMAGE} AS build
WORKDIR /src/web
COPY --from=web-src package.json package-lock.json ./
RUN --mount=type=cache,target=/root/.npm,sharing=locked npm ci --ignore-scripts
COPY --from=web-src . ./
RUN npm run build

FROM ${NGINX_RUNTIME_IMAGE}
COPY --from=ops-src web-proxy/nginx.conf /etc/nginx/nginx.conf
COPY --from=build --chown=101:101 /src/web/dist /usr/share/nginx/html
USER 101:101
EXPOSE 8080
ENTRYPOINT ["nginx", "-g", "daemon off;"]
