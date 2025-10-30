GET http://localhost:8080/api/dev/migration-status
=======================================================
POST http://localhost:8080/api/dev/rollback?steps=1
=======================================================
POST http://localhost:8080/api/dev/rollback?steps=2

Rollback All Migrations (in the code)
err := sqlite.RollbackAll("./social-network.db", "./pkg/db/migrations/sqlite")

### How Rollback Process Works
1- Current State: Version 3 (users, posts, comments tables exist)
2- Rollback 1 step:
- Runs 003_add_comments.down.sql
- New state: Version 2 (users, posts tables exist)
3- Rollback 2 more steps:
- Runs 002_add_posts.down.sql
- Runs 001_create_users.down.sql
- New state: Version 0 (empty database)