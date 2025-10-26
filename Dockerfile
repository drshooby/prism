# Use a base Node.js image
FROM node:22.11.0-alpine

# Set the working directory inside the container
WORKDIR /app

# Copy package.json and yarn.lock (or package-lock.json) first
# This allows Docker to cache the dependency installation step
COPY package.json package-lock.json ./

# Install dependencies
RUN npm install && \
    apk add --no-cache bash curl git terraform tflint || true

# Copy the rest of your application code
COPY . .

# Expose the port Next.js runs on (default 3000)
EXPOSE 3000

# Command to run the Next.js development server
CMD ["npx", "next", "dev", "--turbo"]
