# CHANAKYA System Architecture

## Overview
Chanakya is designed with a decoupled architecture, ensuring that the heavy NLP processing and regulatory ingestion does not block the human-in-the-loop review interface.

### 1. Frontend (Next.js 16)
- **Deployment Strategy**: Dockerized, running in standalone mode for minimal container size.
- **State Management**: React Query with `@tanstack/react-query-persist-client` for aggressive offline caching of workflows.
- **Styling**: TailwindCSS with custom Radix UI primitives.

### 2. Backend (Golang 1.23)
- **Design**: Strictly stateless HTTP APIs.
- **Processing Engine**: PDF text is extracted and chunked locally before being sent to the Gemini API (`GEMINI_API_KEY`) for structural comprehension.
- **Data Persistence**: Currently utilizing SQLite for local zero-config deployments (`chanakya.db`). The abstraction layer natively supports drop-in replacement with PostgreSQL for horizontal scaling.

### 3. AI NLP Pipeline
- **Orchestration**: The Go backend streams partial structural parses directly to the frontend (Server-Sent Events), masking latency with a pipeline visualizer.
- **Generation**: The system leverages Gemini 1.5 Pro/Flash to identify obligations, map cross-references, and synthesize drafting recommendations.

### 4. Scalability & Readiness
- **Rate Limiting**: Integrated via `@upstash/ratelimit`.
- **Caching**: Stale-while-revalidate patterns are heavily utilized on the frontend.
- **Security**: Strict `.env` parsing, no exposed keys, isomorphic DOM purifying for all user inputs.
