FROM node:20-alpine AS base
WORKDIR /app

FROM base AS deps
COPY package*.json ./
RUN npm ci

FROM deps AS dev
ENV NODE_ENV=development
COPY . .
RUN npx prisma generate
EXPOSE 4004
CMD ["sh", "./docker-entrypoint.sh", "npm", "run", "start:dev"]

FROM deps AS build
COPY prisma ./prisma/
COPY prisma.config.ts ./
COPY . .
RUN npx prisma generate
RUN npm run build

FROM base AS prod
ENV NODE_ENV=production
COPY package*.json ./
RUN npm ci --omit=dev && npm cache clean --force
COPY --from=build /app/dist ./dist
COPY --from=build /app/prisma ./prisma
COPY --from=build /app/prisma.config.ts ./prisma.config.ts
COPY docker-entrypoint.sh ./docker-entrypoint.sh
EXPOSE 4004
USER node
CMD ["sh", "./docker-entrypoint.sh", "node", "dist/main"]

FROM dev AS default
