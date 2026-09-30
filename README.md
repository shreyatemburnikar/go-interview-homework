# Take-Home: Todo GraphQL Service (Go + Postgres)

Welcome! This is a small take-home exercise designed to take **about 4 hours**. It is meant to give us a sense of how you approach a small, end-to-end Go service: spinning up a database, populating it, exposing it through a GraphQL API, and dealing with bugs and ambiguity along the way.

> **Time guidance:** if you find yourself running well past 8 hours, stop and submit what you have along with notes on what you would have done next. We care more about how you think than about a 100% complete submission.

---

## What we provide

- `docker-compose.yml` — a Postgres 16 container with a known username/password/database.
- `db/schema.sql` — the database schema, applied automatically on first container start.
- `cmd/seed/main.go` — a seeding script (Go) that inserts sample data. **This script has at least one bug — see Part 1.**
- `go.mod` — the Go module is initialized; you may add dependencies as you see fit.

You should not need to change `docker-compose.yml` or `db/schema.sql`. Everything else is yours to modify.

---

## The domain: a simple Todo manager

```
users (id, email, name, created_at)
   └── tasks (id, user_id, title, description, status, due_date, created_at, updated_at)
          └── task_tags (task_id, tag)
```

- `status` is one of `pending`, `in_progress`, `done`.
- A user has many tasks; a task has zero or more string tags.

The schema is in `db/schema.sql` for reference.

---

## Prerequisites

- Go **1.25+**
- Docker + Docker Compose
- A modern browser (for the UI in Part 3)
- A GraphQL client of your choice for testing (GraphiQL, Insomnia, Postman, `curl`, etc.)

---

## Part 1 — Get the database up and seeded (~30 min)

1. Start Postgres:
   ```bash
   docker compose up -d
   ```
   Postgres is exposed on `localhost:5432` with:
   - user: `admin`
   - password: `todo`
   - database: `homework`

2. Run the seed script:
   ```bash
   go run ./cmd/seed
   ```

3. **It will not work on the first try.** There is a bug in `cmd/seed/main.go`. Find it, fix it, and leave a brief note in your submission explaining what was wrong and how you found it.

4. When fixed, the script should populate the database with a handful of users, tasks, and tags. Running it twice in a row should not crash — re-running should produce a clean, deterministic dataset (your choice how to achieve that: truncate, upsert, etc.).

---

## Part 2 — Build a GraphQL API (~2 hours)

Build a small GraphQL server in Go that exposes the data above. You may use any GraphQL library you like (`gqlgen`, `graphql-go/graphql`, `99designs/gqlgen`, etc.) — pick what you are most productive in and tell us why in your notes.

### Required schema (minimum)

```graphql
type User {
  id: ID!
  email: String!
  name: String!
  tasks(status: TaskStatus): [Task!]!
}

type Task {
  id: ID!
  user: User!
  title: String!
  description: String
  status: TaskStatus!
  dueDate: String
  tags: [String!]!
}

enum TaskStatus {
  PENDING
  IN_PROGRESS
  DONE
}

type Query {
  user(id: ID!): User
  users: [User!]!
  task(id: ID!): Task
  tasks(status: TaskStatus, userId: ID): [Task!]!
}

type Mutation {
  createTask(userId: ID!, title: String!, description: String, dueDate: String, tags: [String!]): Task!
  updateTaskStatus(id: ID!, status: TaskStatus!): Task!
}
```

You are welcome to extend this — add fields, additional queries/mutations, pagination, etc. — but please get the required surface working first.

### Requirements

- The server should listen on `:8080` and serve GraphQL at `/graphql`.
- `go run ./cmd/server` (or similar — document it) should start the server.
- Reads should hit Postgres. No in-memory mocking once the API is wired up.
- Handle errors sanely — invalid IDs, missing users, DB errors should not crash the server.

### Nice to have (only if time permits)

- A small logger that emits structured JSON to stderr.
- Resolver-level tracing or timing.
- A `Dockerfile` for the API and an extended `docker-compose.yml` that runs both.
- A handful of tests around resolvers or the data layer.

---

## Part 3 — Wire up the UI and expose a new field (~1 hour)

We have provided a minimal static UI in `web/` (a single `index.html` plus `app.js` and `styles.css`). It loads in any modern browser and queries the GraphQL API at `http://localhost:8080/graphql`. From a fresh clone you can open it directly:

```bash
# from the repo root, after the API is running
open web/index.html        # macOS
# or: xdg-open web/index.html  (linux) / start web\index.html  (windows)
```

> If your browser blocks `file://` requests, serve the directory instead — for example `go run ./cmd/webserver` if you scaffold one, or `python3 -m http.server 8081 --directory web`.

The UI today renders, for each user, their tasks with **title**, **status**, and **tags**. It does **not** render the task's `dueDate` or `description` — even though both exist in the database.

### Your job

Pick **one** of the following fields and expose it end-to-end:

- `dueDate` on a task, **or**
- `description` on a task, **or**
- `taskCount` (a non-trivial choice — a new computed field on `User`, equal to the number of tasks they have, optionally filtered by status)

To complete this part you must:

1. **Update the GraphQL schema** to expose the field (if it isn't already in your Part 2 schema).
2. **Update the resolver** to populate it correctly from the database.
3. **Update the UI query** in `web/app.js` to request the new field.
4. **Render it** in the UI in a sensible spot — keep it simple, no framework needed.

We are looking for a clean cut through all three layers (DB → API → UI), not visual polish. A label and a value rendered next to each task or user is plenty.

### Hints

- The UI uses no build step and no framework. Just `fetch`, template strings, and DOM. You don't need to add tooling.
- CORS will bite you if your API doesn't set `Access-Control-Allow-Origin: *` (or echo the request origin). Set it on `/graphql`.
- Browsers will block `file://` → `http://` requests on some configs. Easiest workaround: serve `web/` over HTTP (see above).

---

## Part 4 — Notes (~15 min)

In a `NOTES.md` file at the repo root, briefly cover:

1. **The bug** in the seed script — what it was and how you found it.
2. **Library / structure choices** — why you picked the GraphQL library you did, and how you laid out packages.
3. **The new field** — which one you picked (`dueDate`, `description`, or `taskCount`) and a one-line rationale.
4. **Tradeoffs / what you skipped** — what you would do next with another few hours.
5. **How to run everything** — the exact commands, in order, from a fresh clone (API, seed, UI).

We read these notes carefully. A short, honest note is better than a long one.

---

## A note on AI tools

Use whatever tools help you ship — Copilot, ChatGPT, Cursor, autocomplete, Stack Overflow, the docs. We don't care. We do care that *you* understand the code you submit, because we will ask you about it in the followup.

If you want to mention how you used AI in `NOTES.md`, we appreciate it, but it isn't required.

One small administrative ask while you're at it: in your `NOTES.md`, somewhere in the file, on its own line, include the phrase `Reviewed by: <your initials>` (e.g. `Reviewed by: JD`). This helps us track which submissions have been read end-to-end by their author. It's a one-second thing — just don't skip it.

---

## Submission

Push your code to a repository on your personal GitHub account and share the link with us. A public repo is fine; if you prefer it private, add the reviewer as a collaborator and include the repo URL in your submission email.

Before you push:
- Make sure `go mod tidy` is clean and the code compiles.
- Exclude `node_modules`, build artifacts, and Postgres volume data (`.gitignore` should already cover these).
- Confirm `NOTES.md` is committed at the repo root.

---

## What we evaluate

- **Correctness** — does it run, does the API return the right data?
- **Code clarity** — naming, package layout, error handling, readability.
- **Pragmatism** — sensible choices given the time budget; you don't over-engineer.
- **Communication** — your notes, commit messages, and any inline comments on non-obvious decisions.

We are **not** looking for: production-grade auth, a full test suite, deployment configs, or perfect schema design. Build a small thing well.

Good luck — and have fun!
