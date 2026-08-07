# CHANAKYA

Chanakya is an advanced, AI-driven Regulatory Compliance and AI workflow automation platform designed specifically for the financial sector (e.g. SEBI).

## Architecture

This project is organized as a monorepo consisting of:
1. **Frontend**: Next.js 16 (App Router), TailwindCSS, Radix UI primitives.
2. **Backend**: Golang (1.23+), SQLite.
3. **AI Layer**: Deep integration with Google Gemini via `GEMINI_API_KEY`.

### Scaling & Architecture
The system is built to scale horizontally:
- Next.js runs in `standalone` mode, easily dockerized and scaled via load balancers.
- The Golang backend is stateless.
- The SQLite database handles local concurrency, but can be seamlessly migrated to PostgreSQL via GORM for horizontal deployment.

## Getting Started

### Prerequisites
- Node.js (v20+)
- Go (v1.23+)
- Python 3 (For scripting)

### Environment Variables
To run Chanakya, you need a Google Gemini API Key.
Create a `.env.local` in `frontend/apps/web/` and `.env` in `backend/` with the following:
```
GEMINI_API_KEY=your_gemini_api_key
```

### Running Locally

**Start the Backend:**
```bash
cd backend
go run ./cmd/api
```

**Start the Frontend:**
```bash
cd frontend/apps/web
npm install
npm run dev
```

## Features
- **Regulatory Intake**: AI extraction of compliance obligations from complex PDFs.
- **Review Queue**: Human-in-the-loop review system.
- **Workflow Automation**: Auto-generates SOPs and API actions based on regulatory rules.
- **Blast Radius Analysis**: PDF reporting on the impact of new regulations on existing systems.
