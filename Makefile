.PHONY: build run test dev build-frontend

build-frontend:
	cd web && npm run build
	rm -rf cmd/portal/web/dist
	mkdir -p cmd/portal/web
	cp -r web/dist cmd/portal/web/dist

build: build-frontend
	go build -o bin/portal ./cmd/portal

run: build
	./bin/portal

test:
	go test ./...

dev-backend:
	go run ./cmd/portal

dev-frontend:
	cd web && npm run dev
