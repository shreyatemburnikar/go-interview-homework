# Implementation Notes

## Seed Bug

The provided seed script contained an intentional SQL bug in the task insert statement.

The original query used MySQL-style backticks around the `user_id` column:

`INSERT INTO tasks (`user_id`, ...)`

PostgreSQL does not use backticks for identifiers, so the seed command failed with a PostgreSQL syntax error:

`SQLSTATE 42601`

I identified the problem from the error reported while inserting the task "Draft analytical engine notes" and corrected the INSERT statement by removing the backticks.

The corrected statement uses:

`INSERT INTO tasks (user_id, title, description, status, due_date) ...`

I also verified that the seed script can be executed multiple times. The script truncates the existing seed data and resets identities before inserting the records again, making the seed process deterministic.

## GraphQL Implementation

I used `github.com/graphql-go/graphql` to implement the GraphQL API.

I chose this library because it provides a relatively lightweight code-first GraphQL implementation and works well with Go's standard `database/sql` package. For the scope of this take-home assignment, it allowed the GraphQL schema and resolvers to remain straightforward without introducing code generation.

The server is implemented in:

`cmd/server/main.go`

The GraphQL endpoint is:

`http://localhost:8080/graphql`

The API supports the required queries:

- `user(id: ID!)`
- `users`
- `task(id: ID!)`
- `tasks(status: TaskStatus, userId: ID)`

The API also supports the required mutations:

- `createTask`
- `updateTaskStatus`

Relationships are resolved through:

- `User.tasks`
- `Task.user`
- `Task.tags`

The `User.tasks` field also supports filtering by task status.

The server accepts both GET and POST GraphQL requests. POST requests support JSON request bodies and GraphQL variables.

CORS headers are included so the static frontend can access the GraphQL endpoint running on port 8080.

## Part 3 - UI Field

For the database-to-API-to-UI change, I chose the task `dueDate` field.

The value originates from the `tasks.due_date` column in PostgreSQL.

The GraphQL `Task` type exposes this value as:

`dueDate: String`

The frontend GraphQL query in `web/app.js` requests `dueDate`, and the task renderer displays either:

`Due: YYYY-MM-DD`

or:

`No due date`

This demonstrates the complete flow:

PostgreSQL `due_date`
→ Go Task model
→ GraphQL `dueDate`
→ frontend GraphQL query
→ browser UI

## Tradeoffs

For this take-home assignment I kept the implementation intentionally small and direct.

Database queries are implemented directly in the GraphQL resolvers using `database/sql`. This avoids adding an ORM or repository abstraction for a small application, although a larger production service would likely separate database access, business logic, and GraphQL resolvers into different packages.

The GraphQL schema is defined programmatically using `graphql-go/graphql`. This keeps setup simple, but a larger application could benefit from a schema-first/code-generation solution such as gqlgen for stronger generated typing and easier maintenance of a larger schema.

The `createTask` mutation uses a database transaction so creation of the task and its tags is atomic.

The server uses the `DATABASE_URL` environment variable when provided and otherwise falls back to the local Docker Compose PostgreSQL connection used by the assignment.

## Running the Application

Docker Desktop must be running.

From the project root, start PostgreSQL:

```bash
docker compose up -d