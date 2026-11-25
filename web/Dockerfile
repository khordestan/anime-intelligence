FROM node:22-alpine AS deps
WORKDIR /app
COPY package*.json ./
RUN npm ci
FROM node:22-alpine AS build
WORKDIR /app
COPY --from=deps /app/node_modules ./node_modules
COPY . .
ARG NEXT_PUBLIC_API_BASE_URL=http://localhost:8080/api/v1
RUN npm run build
FROM node:22-alpine
WORKDIR /app
ENV NODE_ENV=production
COPY --from=build --chown=node:node /app ./
USER node
EXPOSE 3000
CMD ["npm","run","start"]
