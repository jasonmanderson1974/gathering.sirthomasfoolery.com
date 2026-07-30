# Build stage
FROM node:18-alpine AS builder

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
