# Build stage
#
# Node 20, matching every CI workflow (backend-ci, frontend-ci, browser-ci all
# pin node-version: 20). It was 18 here, which is a different toolchain from the
# one CI and the deploy box build with — and it bit: workbox-webpack-plugin
# pulls in a package that reads the global `crypto`, which only exists from
# Node 19, so the service worker built fine everywhere except in this image.
# Keep this in step with the workflows.
FROM node:20-alpine AS builder

WORKDIR /app

# Copy package files
COPY package*.json ./

# Install dependencies
RUN npm ci

# Copy source code
COPY . .

# Build arguments for frontend configuration
ARG VUE_APP_GOOGLE_CLIENT_ID=""
ARG VUE_APP_MICROSOFT_CLIENT_ID=""
# Optional: address suggestions in the event location field. Unset = plain text field.
ARG VUE_APP_GOOGLE_MAPS_API_KEY=""

ENV VUE_APP_GOOGLE_CLIENT_ID=$VUE_APP_GOOGLE_CLIENT_ID
ENV VUE_APP_MICROSOFT_CLIENT_ID=$VUE_APP_MICROSOFT_CLIENT_ID
ENV VUE_APP_GOOGLE_MAPS_API_KEY=$VUE_APP_GOOGLE_MAPS_API_KEY

# Build the app and move to /build so it's not hidden by volume mount
RUN npm run build && mv dist /build

# Output stage - just the dist folder
FROM scratch AS dist
COPY --from=builder /build /dist
