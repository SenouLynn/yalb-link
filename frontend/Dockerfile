# Frontend development server. This image is not a production static build.
#
# Node 22 matches the version CI installs (.github/workflows/ci.yml). pnpm is
# not pinned here — package.json's packageManager field owns that, and corepack
# reads it.
ARG NODE_VERSION=22

FROM node:${NODE_VERSION}-alpine

RUN corepack enable

WORKDIR /app

COPY package.json pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile

COPY . .

EXPOSE 3000

# --host binds 0.0.0.0; without it Vite listens on loopback inside the
# container and the published port answers nothing.
CMD ["pnpm", "dev", "--host", "0.0.0.0", "--port", "3000"]
