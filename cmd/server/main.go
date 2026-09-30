package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/graphql-go/graphql"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type User struct {
	ID    int64  `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

type Task struct {
	ID          int64   `json:"id"`
	UserID      int64   `json:"userId"`
	Title       string  `json:"title"`
	Description *string `json:"description"`
	Status      string  `json:"status"`
	DueDate     *string `json:"dueDate"`
}

type graphQLRequest struct {
	Query     string                 `json:"query"`
	Variables map[string]interface{} `json:"variables"`
}

func main() {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://admin:todo@localhost:5432/homework?sslmode=disable"
	}

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatal(err)
	}

	fmt.Println("Connected to PostgreSQL!")

	taskStatusType := graphql.NewEnum(
		graphql.EnumConfig{
			Name: "TaskStatus",
			Values: graphql.EnumValueConfigMap{
				"PENDING": &graphql.EnumValueConfig{
					Value: "pending",
				},
				"IN_PROGRESS": &graphql.EnumValueConfig{
					Value: "in_progress",
				},
				"DONE": &graphql.EnumValueConfig{
					Value: "done",
				},
			},
		},
	)

	var userType *graphql.Object
	var taskType *graphql.Object

	userType = graphql.NewObject(
		graphql.ObjectConfig{
			Name: "User",
			Fields: graphql.FieldsThunk(func() graphql.Fields {
				return graphql.Fields{
					"id": &graphql.Field{
						Type: graphql.NewNonNull(graphql.ID),
					},
					"email": &graphql.Field{
						Type: graphql.NewNonNull(graphql.String),
					},
					"name": &graphql.Field{
						Type: graphql.NewNonNull(graphql.String),
					},
					"tasks": &graphql.Field{
						Type: graphql.NewNonNull(
							graphql.NewList(
								graphql.NewNonNull(taskType),
							),
						),
						Args: graphql.FieldConfigArgument{
							"status": &graphql.ArgumentConfig{
								Type: taskStatusType,
							},
						},
						Resolve: func(p graphql.ResolveParams) (interface{}, error) {
							user, ok := p.Source.(User)
							if !ok {
								return nil, errors.New("invalid user source")
							}

							status, hasStatus := p.Args["status"]

							var rows *sql.Rows
							var err error

							if hasStatus {
								rows, err = db.Query(`
									SELECT
										id,
										user_id,
										title,
										description,
										status,
										due_date::text
									FROM tasks
									WHERE user_id = $1
									  AND status = $2
									ORDER BY id
								`, user.ID, status)
							} else {
								rows, err = db.Query(`
									SELECT
										id,
										user_id,
										title,
										description,
										status,
										due_date::text
									FROM tasks
									WHERE user_id = $1
									ORDER BY id
								`, user.ID)
							}

							if err != nil {
								return nil, err
							}
							defer rows.Close()

							return scanTasks(rows)
						},
					},
				}
			}),
		},
	)

	taskType = graphql.NewObject(
		graphql.ObjectConfig{
			Name: "Task",
			Fields: graphql.FieldsThunk(func() graphql.Fields {
				return graphql.Fields{
					"id": &graphql.Field{
						Type: graphql.NewNonNull(graphql.ID),
					},
					"user": &graphql.Field{
						Type: graphql.NewNonNull(userType),
						Resolve: func(p graphql.ResolveParams) (interface{}, error) {
							task, ok := p.Source.(Task)
							if !ok {
								return nil, errors.New("invalid task source")
							}

							var user User

							err := db.QueryRow(`
								SELECT id, email, name
								FROM users
								WHERE id = $1
							`, task.UserID).Scan(
								&user.ID,
								&user.Email,
								&user.Name,
							)

							if err != nil {
								return nil, err
							}

							return user, nil
						},
					},
					"title": &graphql.Field{
						Type: graphql.NewNonNull(graphql.String),
					},
					"description": &graphql.Field{
						Type: graphql.String,
					},
					"status": &graphql.Field{
						Type: graphql.NewNonNull(taskStatusType),
					},
					"dueDate": &graphql.Field{
						Type: graphql.String,
					},
					"tags": &graphql.Field{
						Type: graphql.NewNonNull(
							graphql.NewList(
								graphql.NewNonNull(graphql.String),
							),
						),
						Resolve: func(p graphql.ResolveParams) (interface{}, error) {
							task, ok := p.Source.(Task)
							if !ok {
								return nil, errors.New("invalid task source")
							}

							rows, err := db.Query(`
								SELECT tag
								FROM task_tags
								WHERE task_id = $1
								ORDER BY tag
							`, task.ID)

							if err != nil {
								return nil, err
							}
							defer rows.Close()

							tags := []string{}

							for rows.Next() {
								var tag string

								if err := rows.Scan(&tag); err != nil {
									return nil, err
								}

								tags = append(tags, tag)
							}

							if err := rows.Err(); err != nil {
								return nil, err
							}

							return tags, nil
						},
					},
				}
			}),
		},
	)

	queryFields := graphql.Fields{
		"user": &graphql.Field{
			Type: userType,
			Args: graphql.FieldConfigArgument{
				"id": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.ID),
				},
			},
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				id, err := parseID(p.Args["id"])
				if err != nil {
					return nil, err
				}

				var user User

				err = db.QueryRow(`
					SELECT id, email, name
					FROM users
					WHERE id = $1
				`, id).Scan(
					&user.ID,
					&user.Email,
					&user.Name,
				)

				if errors.Is(err, sql.ErrNoRows) {
					return nil, nil
				}
				if err != nil {
					return nil, err
				}

				return user, nil
			},
		},

		"users": &graphql.Field{
			Type: graphql.NewNonNull(
				graphql.NewList(
					graphql.NewNonNull(userType),
				),
			),
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				rows, err := db.Query(`
					SELECT id, email, name
					FROM users
					ORDER BY id
				`)
				if err != nil {
					return nil, err
				}
				defer rows.Close()

				users := []User{}

				for rows.Next() {
					var user User

					if err := rows.Scan(
						&user.ID,
						&user.Email,
						&user.Name,
					); err != nil {
						return nil, err
					}

					users = append(users, user)
				}

				if err := rows.Err(); err != nil {
					return nil, err
				}

				return users, nil
			},
		},

		"task": &graphql.Field{
			Type: taskType,
			Args: graphql.FieldConfigArgument{
				"id": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.ID),
				},
			},
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				id, err := parseID(p.Args["id"])
				if err != nil {
					return nil, err
				}

				task, err := getTaskByID(db, id)

				if errors.Is(err, sql.ErrNoRows) {
					return nil, nil
				}
				if err != nil {
					return nil, err
				}

				return task, nil
			},
		},

		"tasks": &graphql.Field{
			Type: graphql.NewNonNull(
				graphql.NewList(
					graphql.NewNonNull(taskType),
				),
			),
			Args: graphql.FieldConfigArgument{
				"status": &graphql.ArgumentConfig{
					Type: taskStatusType,
				},
				"userId": &graphql.ArgumentConfig{
					Type: graphql.ID,
				},
			},
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				status, hasStatus := p.Args["status"]
				userIDValue, hasUserID := p.Args["userId"]

				var rows *sql.Rows
				var err error

				switch {
				case hasStatus && hasUserID:
					userID, err := parseID(userIDValue)
					if err != nil {
						return nil, err
					}

					rows, err = db.Query(`
						SELECT
							id,
							user_id,
							title,
							description,
							status,
							due_date::text
						FROM tasks
						WHERE status = $1
						  AND user_id = $2
						ORDER BY id
					`, status, userID)

				case hasStatus:
					rows, err = db.Query(`
						SELECT
							id,
							user_id,
							title,
							description,
							status,
							due_date::text
						FROM tasks
						WHERE status = $1
						ORDER BY id
					`, status)

				case hasUserID:
					userID, err := parseID(userIDValue)
					if err != nil {
						return nil, err
					}

					rows, err = db.Query(`
						SELECT
							id,
							user_id,
							title,
							description,
							status,
							due_date::text
						FROM tasks
						WHERE user_id = $1
						ORDER BY id
					`, userID)

				default:
					rows, err = db.Query(`
						SELECT
							id,
							user_id,
							title,
							description,
							status,
							due_date::text
						FROM tasks
						ORDER BY id
					`)
				}

				if err != nil {
					return nil, err
				}
				defer rows.Close()

				return scanTasks(rows)
			},
		},
	}

	mutationFields := graphql.Fields{
		"createTask": &graphql.Field{
			Type: graphql.NewNonNull(taskType),
			Args: graphql.FieldConfigArgument{
				"userId": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.ID),
				},
				"title": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.String),
				},
				"description": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
				"dueDate": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
				"tags": &graphql.ArgumentConfig{
					Type: graphql.NewList(
						graphql.NewNonNull(graphql.String),
					),
				},
			},
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				userID, err := parseID(p.Args["userId"])
				if err != nil {
					return nil, err
				}

				title, _ := p.Args["title"].(string)

				var description interface{}
				if value, ok := p.Args["description"].(string); ok {
					description = value
				}

				var dueDate interface{}
				if value, ok := p.Args["dueDate"].(string); ok {
					dueDate = value
				}

				tx, err := db.Begin()
				if err != nil {
					return nil, err
				}

				defer tx.Rollback()

				var taskID int64

				err = tx.QueryRow(`
					INSERT INTO tasks (
						user_id,
						title,
						description,
						due_date
					)
					VALUES ($1, $2, $3, $4)
					RETURNING id
				`,
					userID,
					title,
					description,
					dueDate,
				).Scan(&taskID)

				if err != nil {
					return nil, err
				}

				if tagsValue, ok := p.Args["tags"]; ok {
					tags, ok := tagsValue.([]interface{})
					if !ok {
						return nil, errors.New("invalid tags")
					}

					for _, tagValue := range tags {
						tag, ok := tagValue.(string)
						if !ok {
							return nil, errors.New("invalid tag")
						}

						_, err := tx.Exec(`
							INSERT INTO task_tags (task_id, tag)
							VALUES ($1, $2)
							ON CONFLICT DO NOTHING
						`, taskID, tag)

						if err != nil {
							return nil, err
						}
					}
				}

				if err := tx.Commit(); err != nil {
					return nil, err
				}

				return getTaskByID(db, taskID)
			},
		},

		"updateTaskStatus": &graphql.Field{
			Type: graphql.NewNonNull(taskType),
			Args: graphql.FieldConfigArgument{
				"id": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.ID),
				},
				"status": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(taskStatusType),
				},
			},
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				id, err := parseID(p.Args["id"])
				if err != nil {
					return nil, err
				}

				status, ok := p.Args["status"].(string)
				if !ok {
					return nil, errors.New("invalid task status")
				}

				result, err := db.Exec(`
					UPDATE tasks
					SET status = $1,
					    updated_at = now()
					WHERE id = $2
				`, status, id)

				if err != nil {
					return nil, err
				}

				count, err := result.RowsAffected()
				if err != nil {
					return nil, err
				}

				if count == 0 {
					return nil, fmt.Errorf("task %d not found", id)
				}

				return getTaskByID(db, id)
			},
		},
	}

	rootQuery := graphql.NewObject(
		graphql.ObjectConfig{
			Name:   "Query",
			Fields: queryFields,
		},
	)

	rootMutation := graphql.NewObject(
		graphql.ObjectConfig{
			Name:   "Mutation",
			Fields: mutationFields,
		},
	)

	schema, err := graphql.NewSchema(
		graphql.SchemaConfig{
			Query:    rootQuery,
			Mutation: rootMutation,
		},
	)
	if err != nil {
		log.Fatal(err)
	}

	http.HandleFunc("/graphql", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Content-Type", "application/json")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		var query string
		var variables map[string]interface{}

		switch r.Method {
		case http.MethodGet:
			query = r.URL.Query().Get("query")

		case http.MethodPost:
			var request graphQLRequest

			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				http.Error(w, `{"error":"invalid JSON request"}`, http.StatusBadRequest)
				return
			}

			query = request.Query
			variables = request.Variables

		default:
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}

		result := graphql.Do(
			graphql.Params{
				Schema:         schema,
				RequestString:  query,
				VariableValues: variables,
			},
		)

		if err := json.NewEncoder(w).Encode(result); err != nil {
			log.Println("encode response:", err)
		}
	})

	fmt.Println("GraphQL server running at http://localhost:8080/graphql")

	log.Fatal(http.ListenAndServe(":8080", nil))
}

func parseID(value interface{}) (int64, error) {
	switch v := value.(type) {
	case string:
		return strconv.ParseInt(v, 10, 64)

	case int:
		return int64(v), nil

	case int64:
		return v, nil

	default:
		return 0, fmt.Errorf("invalid ID: %v", value)
	}
}

func scanTasks(rows *sql.Rows) ([]Task, error) {
	tasks := []Task{}

	for rows.Next() {
		var task Task

		if err := rows.Scan(
			&task.ID,
			&task.UserID,
			&task.Title,
			&task.Description,
			&task.Status,
			&task.DueDate,
		); err != nil {
			return nil, err
		}

		tasks = append(tasks, task)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return tasks, nil
}

func getTaskByID(db *sql.DB, id int64) (Task, error) {
	var task Task

	err := db.QueryRow(`
		SELECT
			id,
			user_id,
			title,
			description,
			status,
			due_date::text
		FROM tasks
		WHERE id = $1
	`, id).Scan(
		&task.ID,
		&task.UserID,
		&task.Title,
		&task.Description,
		&task.Status,
		&task.DueDate,
	)

	return task, err
}