ARG NODE_VERSION
FROM node:${NODE_VERSION}
WORKDIR /renderer
RUN apk add --no-cache chromium font-noto && \
    npm install --omit=dev --no-audit --no-fund --no-package-lock playwright-core@1.62.1
RUN apk add --no-cache font-noto-arabic font-noto-cjk font-noto-devanagari
COPY integrations/website-renderer/*.cjs ./
USER node
ENTRYPOINT ["node", "/renderer/render.cjs"]
