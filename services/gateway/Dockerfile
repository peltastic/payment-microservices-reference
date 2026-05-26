FROM node:20-alpine AS base
WORKDIR /app

FROM base AS deps
COPY package*.json ./
RUN npm ci

FROM deps AS dev
ENV NODE_ENV=development
COPY . .
EXPOSE 3000
CMD ["npm", "run", "start:dev"]

FROM deps AS build
COPY . .
RUN npm run build

FROM base AS prod
ENV NODE_ENV=production
COPY package*.json ./
RUN npm ci --omit=dev && npm cache clean --force
COPY --from=build /app/dist ./dist
EXPOSE 3000
USER node
CMD ["node", "dist/main"]

FROM dev AS default
