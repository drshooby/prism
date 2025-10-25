# Create T3 App

This is a [T3 Stack](https://create.t3.gg/) project bootstrapped with `create-t3-app`.

## Getting Started

### Environment Setup

1. Copy the example environment file:
   ```bash
   cp .env.example .env
   ```

2. Fill in the required environment variables in `.env` (see Infisical setup below for `INFISICAL_CLIENT_ID` and `INFISICAL_CLIENT_SECRET`)

### Infisical Setup

Before running the full application, you need to set up Infisical for secrets management:

1. Start the Infisical service:
   ```bash
   docker compose up infisical-secrets-manager infisical-db infisical-redis
   ```

2. Access Infisical at `http://localhost:8080` and complete the setup

3. Configure a machine identity (required for the Go SDK):
   - Go to **Org** → **Access Control** → **Identities** → Create Identity with **Member Role**
   - Go to **Secrets Manager** → **Access Management** → **Machine Identities** → Add Identity → Select with **Developer Role**
   - Go to **Org** → **Access Control** → **Identities** → Click your Identity → **Universal Auth**
   - Copy the **Client ID** and create a **Client Secret**, then copy it
   - Add these values to your `.env` file as `INFISICAL_CLIENT_ID` and `INFISICAL_CLIENT_SECRET`

4. Create a new Project "prism-internal"
5. Create a new key/value secret under the "Development" environment

### Running the Application

Once Infisical is configured, start all services:

```bash
docker compose up
```

This will start:
- PostgreSQL database (port 5432)
- MinIO object storage (ports 9000, 9001)
- Infisical secrets manager (port 8080)
- Go service (port 1323)
- Next.js application (port 3000)

## What's next? How do I make an app with this?

We try to keep this project as simple as possible, so you can start with just the scaffolding we set up for you, and add additional things later when they become necessary.

If you are not familiar with the different technologies used in this project, please refer to the respective docs. If you still are in the wind, please join our [Discord](https://t3.gg/discord) and ask for help.

- [Next.js](https://nextjs.org)
- [NextAuth.js](https://next-auth.js.org)
- [Prisma](https://prisma.io)
- [Drizzle](https://orm.drizzle.team)
- [Tailwind CSS](https://tailwindcss.com)
- [tRPC](https://trpc.io)

## Learn More

To learn more about the [T3 Stack](https://create.t3.gg/), take a look at the following resources:

- [Documentation](https://create.t3.gg/)
- [Learn the T3 Stack](https://create.t3.gg/en/faq#what-learning-resources-are-currently-available) — Check out these awesome tutorials

You can check out the [create-t3-app GitHub repository](https://github.com/t3-oss/create-t3-app) — your feedback and contributions are welcome!

## How do I deploy this?

Follow our deployment guides for [Vercel](https://create.t3.gg/en/deployment/vercel), [Netlify](https://create.t3.gg/en/deployment/netlify) and [Docker](https://create.t3.gg/en/deployment/docker) for more information.
