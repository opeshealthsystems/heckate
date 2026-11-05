FROM node:22-alpine AS build
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci
COPY . .
ARG NEXT_PUBLIC_SITE_URL=https://hekatesystems.com
ARG NEXT_PUBLIC_API_URL=https://api.hekatesystems.com
ARG NEXT_PUBLIC_PORTAL_URL=https://app.hekatesystems.com
ENV NEXT_PUBLIC_SITE_URL=$NEXT_PUBLIC_SITE_URL NEXT_PUBLIC_API_URL=$NEXT_PUBLIC_API_URL NEXT_PUBLIC_PORTAL_URL=$NEXT_PUBLIC_PORTAL_URL
RUN npm run build

FROM nginxinc/nginx-unprivileged:1.27-alpine
COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/out /usr/share/nginx/html
USER 101
EXPOSE 8080
