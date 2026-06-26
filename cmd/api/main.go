package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"creator-pipeline/internal/app"
	"creator-pipeline/internal/config"
	creator "creator-pipeline/internal/eino"
	"creator-pipeline/internal/idempotency"
	"creator-pipeline/internal/llm"
	"creator-pipeline/internal/metrics"
	"creator-pipeline/internal/queue"
	"creator-pipeline/internal/storage"
	"creator-pipeline/internal/task"
	"creator-pipeline/internal/video"
	"creator-pipeline/internal/worker"
)

func main() {
	ctx := context.Background()
	cfg := config.Load()

	var filler creator.JSONFiller
	strictMiniMax := cfg.LLMRequired || cfg.MiniMaxStrictMode
	if strictMiniMax && cfg.LLMConfigPath == "" {
		log.Fatal("strict MiniMax mode requires LLM_CONFIG_PATH")
	}
	if cfg.LLMConfigPath != "" {
		llmCfg, err := llm.LoadConfig(cfg.LLMConfigPath)
		if err != nil {
			log.Fatalf("load llm config: %v", err)
		}
		if err := llmCfg.ValidateMiniMax(strictMiniMax); err != nil {
			log.Fatalf("validate minimax config: %v", err)
		}
		filler = llm.NewMiniMaxClient(llmCfg)
		log.Printf("LLM planner enabled provider=%s model=%s strict=%t", llmCfg.Provider, llmCfg.Model, strictMiniMax)
	}

	planner, err := creator.NewPlanner(ctx, filler, creator.WithRequiredLLM(strictMiniMax))
	if err != nil {
		log.Fatalf("build planner: %v", err)
	}

	repo, q, store, idem := buildRuntime(ctx, cfg)
	m := metrics.NewRegistry(q)

	svc := app.NewService(planner, repo, q, idem, m)
	w := worker.New(repo, q, store, video.NewMockGenerator(), m, worker.Config{
		Concurrency: cfg.WorkerConcurrency,
		JobTimeout:  cfg.JobTimeout,
		MaxRetries:  2,
	})
	go w.Run(ctx)

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           routes(svc, repo, m),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("CreatorPipeline API listening on %s infra=%s", server.Addr, cfg.InfraMode)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("server: %v", err)
	}
}

func buildRuntime(ctx context.Context, cfg config.Config) (task.Repository, queue.Queue, storage.Store, idempotency.Store) {
	if cfg.InfraMode != "real" {
		return task.NewMemoryRepository(),
			queue.NewMemoryQueue(),
			storage.NewMockStore(cfg.CDNBaseURL),
			idempotency.NewMemoryStore(10 * time.Minute)
	}

	db, err := sql.Open("mysql", cfg.MySQLDSN)
	if err != nil {
		log.Fatalf("open mysql: %v", err)
	}
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)
	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("connect mysql: %v", err)
	}
	if err := task.EnsureMySQLSchema(ctx, db); err != nil {
		log.Fatalf("ensure mysql schema: %v", err)
	}

	rabbit, err := queue.NewRabbitMQ(cfg.RabbitMQURL)
	if err != nil {
		log.Fatalf("connect rabbitmq: %v", err)
	}

	redisClient := idempotency.NewRedisClient(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Fatalf("connect redis: %v", err)
	}

	minioStore, err := storage.NewMinIOStore(
		cfg.MinIOEndpoint,
		cfg.MinIOAccessKey,
		cfg.MinIOSecretKey,
		cfg.MinIOBucket,
		cfg.MinIOUseSSL,
		cfg.CDNBaseURL,
	)
	if err != nil {
		log.Fatalf("connect minio: %v", err)
	}
	if err := minioStore.EnsureBucket(ctx); err != nil {
		log.Fatalf("ensure minio bucket: %v", err)
	}

	return task.NewMySQLRepository(db),
		rabbit,
		minioStore,
		idempotency.NewRedisStore(redisClient, 10*time.Minute)
}

func routes(svc *app.Service, repo task.Repository, m *metrics.Registry) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /api/v1/creations", func(w http.ResponseWriter, r *http.Request) {
		var req app.CreateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}

		resp, err := svc.CreateCreation(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, "create_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusAccepted, resp)
	})

	mux.HandleFunc("GET /api/v1/tasks/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/tasks/"), "/")
		parts := strings.Split(path, "/")
		if len(parts) == 0 || parts[0] == "" {
			writeError(w, http.StatusNotFound, "not_found", "missing task id")
			return
		}
		if len(parts) == 2 {
			switch parts[1] {
			case "script":
				doc, err := svc.ScriptDocument(r.Context(), parts[0])
				if err != nil {
					writeAppError(w, err, "script_unavailable")
					return
				}
				writeJSON(w, http.StatusOK, doc)
				return
			case "script.md":
				md, err := svc.ScriptMarkdown(r.Context(), parts[0])
				if err != nil {
					writeAppError(w, err, "script_unavailable")
					return
				}
				w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(md))
				return
			}
		}
		if len(parts) != 1 {
			writeError(w, http.StatusNotFound, "not_found", "unknown task resource")
			return
		}
		t, err := repo.Get(r.Context(), parts[0])
		if err != nil {
			writeError(w, http.StatusNotFound, "not_found", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, t)
	})

	mux.HandleFunc("POST /api/v1/tasks/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/v1/tasks/")
		parts := strings.Split(strings.Trim(path, "/"), "/")
		if len(parts) != 2 {
			writeError(w, http.StatusNotFound, "not_found", "expected /api/v1/tasks/{id}/retry or /cancel")
			return
		}

		switch parts[1] {
		case "retry":
			t, err := svc.Retry(r.Context(), parts[0])
			if err != nil {
				writeError(w, http.StatusBadRequest, "retry_failed", err.Error())
				return
			}
			writeJSON(w, http.StatusAccepted, t)
		case "cancel":
			t, err := svc.Cancel(r.Context(), parts[0])
			if err != nil {
				writeError(w, http.StatusBadRequest, "cancel_failed", err.Error())
				return
			}
			writeJSON(w, http.StatusOK, t)
		case "rewrite-shot":
			var req app.RewriteRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
				return
			}
			doc, err := svc.RewriteShot(r.Context(), parts[0], req)
			if err != nil {
				writeAppError(w, err, "rewrite_failed")
				return
			}
			writeJSON(w, http.StatusOK, doc)
		case "rewrite-dialogue":
			var req app.RewriteRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
				return
			}
			doc, err := svc.RewriteDialogue(r.Context(), parts[0], req)
			if err != nil {
				writeAppError(w, err, "rewrite_failed")
				return
			}
			writeJSON(w, http.StatusOK, doc)
		default:
			writeError(w, http.StatusNotFound, "not_found", "unknown task action")
		}
	})

	mux.HandleFunc("GET /metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		_, _ = w.Write([]byte(m.Prometheus()))
	})

	return withCORS(mux)
}

func writeAppError(w http.ResponseWriter, err error, code string) {
	status := http.StatusBadRequest
	if errors.Is(err, app.ErrTaskNotFound) {
		status = http.StatusNotFound
	} else if errors.Is(err, app.ErrScriptNotReady) {
		status = http.StatusConflict
	}
	writeError(w, status, code, err.Error())
}
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code string, message string) {
	writeJSON(w, status, map[string]string{"code": code, "message": message})
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "http://localhost:5173" || origin == "http://127.0.0.1:5173" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
