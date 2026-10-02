# Migration targets. The server also migrates on boot (db.MigrateUp in
# maya-gateway/main.go); these are for migrating without booting, and for CI.

MIGRATIONS := maya-gateway/db/migrations
ENV_FILE   := maya-gateway/.env

# migrate parses the database value as a URL, so the password must be
# percent-encoded. Without this an unescaped @ or / in it silently truncates
# the connection string and points at the wrong host.
DATABASE_URL = $(shell . ./$(ENV_FILE) 2>/dev/null; \
	printf 'postgres://%s:%s@%s:%s/%s?sslmode=disable' \
		"$$DB_USER" \
		"$$(printf '%s' "$$DB_PASSWORD" | jq -sRr @uri)" \
		"$$DB_HOST" "$$DB_PORT" "$$DB_NAME")

MIGRATE = migrate -path $(MIGRATIONS) -database "$(DATABASE_URL)"

# -1 is "no version applied". Plain `force 0` fails: 0 is a real version number,
# so migrate reports `no migration found for version 0`. The -- is required or
# migrate's flag parser rejects -1 as an unknown flag.
#
# Force only when the ledger is wrong. Forcing while the table exists makes
# `up` fail on `relation already exists`, which re-dirties the ledger.
# V=1 clears a dirty flag without discarding the recorded version.
V ?= -1

.PHONY: help migrate-up migrate-down migrate-force migrate-version migrate-create

help: ## List available targets
	@grep -hE '^[a-z-]+:.*?## ' $(MAKEFILE_LIST) | \
		awk -F':.*?## ' '{printf "  %-16s %s\n", $$1, $$2}'

migrate-up: ## Apply all pending migrations
	$(MIGRATE) up

migrate-down: ## Roll back the most recent migration
	$(MIGRATE) down

migrate-force: ## Reset the ledger (V=-1 resets to none, V=1 clears a dirty flag)
	$(MIGRATE) force -- $(V)

migrate-version: ## Print the recorded migration version
	$(MIGRATE) version

# -dir is required or migrate writes into the current directory instead of the
# migrations folder. -seq is deliberately omitted: it derives the next number
# from the version in the database rather than the files on disk, so a stale
# ledger hands back a number that already exists. Without it a UTC timestamp is
# used, which cannot collide.
migrate-create: ## Create a migration pair (make migrate-create M=add_refunds)
	$(MIGRATE) create -ext sql -digits 6 -dir $(MIGRATIONS) $(M)