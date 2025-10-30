# Variables
DB_PATH=./social-network.db
MIGRATIONS_DIR=./pkg/db/migrations/sqlite
MIGRATE_CMD=go run ./pkg/db/migrations/migrate.go

#  phony targets (are not files)
# BUILD
.PHONY: build-migrate build-server
build-migrate:
	go build -o bin/migrate.exe ./db/migrations/migrate.go

build-server:
	go build -o bin/server.exe ./server.go

# Migration commands
.PHONY: migrate-up migrate-down migrate-status migrate-rollback migrate-force

migrate-up:
	$(MIGRATE_CMD) -action=up -db=$(DB_PATH) -migrations=$(MIGRATIONS_DIR)

migrate-down:
	$(MIGRATE_CMD) -action=down -steps=1 -db=$(DB_PATH) -migrations=$(MIGRATIONS_DIR)

migrate-status:
	$(MIGRATE_CMD) -action=status -db=$(DB_PATH) -migrations=$(MIGRATIONS_DIR)

migrate-rollback:
	$(MIGRATE_CMD) -action=rollback -steps=$(STEPS) -db=$(DB_PATH) -migrations=$(MIGRATIONS_DIR)

migrate-rollback-version:
	$(MIGRATE_CMD) -action=rollback -version=$(VERSION) -db=$(DB_PATH) -migrations=$(MIGRATIONS_DIR)

migrate-force:
	$(MIGRATE_CMD) -action=force -version=$(VERSION) -db=$(DB_PATH) -migrations=$(MIGRATIONS_DIR)

# Development
.PHONY: dev-backend dev-frontend
dev-backend:
	go run server.go

dev-frontend:
	cd ../frontend && npm run dev

# Clean
.PHONY: clean
clean:
	rm -rf bin/