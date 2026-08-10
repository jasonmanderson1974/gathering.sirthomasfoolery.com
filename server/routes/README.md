# routes
This package contains all the routes for The Fellowship API

To view the docs, visit http://localhost:3002/swagger/index.html

## How to document routes
Visit https://github.com/swaggo/swag for a comprehensive overview of the swagger comment structure

To generate swagger docs, make sure you have swag installed — pinned to the version in `go.mod`,
not `@latest`:
```
go install github.com/swaggo/swag/cmd/swag@v1.16.1
```
(Its `--version` misreports as v1.8.12. Ignore that.)

Then, in `server/`, run it every time you change a route comment. **Both flags are required:**
```
swag init --parseDependency --parseInternal
```
A bare `swag init` aborts with `cannot find type definition: primitive.DateTime` — swag can't
introspect the Mongo driver types the allowlist models use, and `--parseDependency` resolves them.