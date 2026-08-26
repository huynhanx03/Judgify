# Contributing

Thanks for your interest in contributing! This guide will help you get started.

## Prerequisites

- **Go** 1.25+
- **Node.js** 22+
- **Docker** & Docker Compose
- **Make** (build tool)

## Local Setup

1. **Fork & clone** the repository
2. **Start infrastructure**:
   ```bash
   make docker-up
   ```
3. **Prepare database**:
   ```bash
   make migrate-apply
   make seed
   ```
4. **Run backend**:
   ```bash
   make run-api
   ```
5. **Run frontend** (in another terminal):
   ```bash
   make run-website
   ```

## Development

### Backend (Go)

- Format code: `cd api && gofmt -w .`
- Lint: `cd api && golangci-lint run`
- Test: `cd api && go test ./...`
- Keep deps clean: `cd api && go mod tidy`

### Frontend (Next.js)

- Lint: `cd website && npx eslint .`
- Type check: `cd website && npx tsc --noEmit`
- Build: `cd website && npm run build`

## Pull Request Process

1. Create a branch from `main`
2. Make your changes
3. Ensure CI checks pass (automated on PR)
4. Fill in the PR template
5. Wait for CodeRabbit AI review and address feedback

## Code Style

- **Go**: Follow standard Go conventions, run `gofmt`
- **TypeScript/React**: Follow existing patterns in the codebase, ESLint rules enforced
- **Commits**: Use conventional commit format (`feat:`, `fix:`, `chore:`, etc.)

## Need Help?

Open a [Discussion](https://github.com/huynhanx03/Judgify/discussions) or create an issue.
