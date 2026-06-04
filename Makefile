APP_NAME=github-cleaner

run:
	go run main.go

docker-up:
	docker compose up -d

docker-down:
	docker compose down

docker-restart:
	docker compose down
	docker compose up -d

metrics:
	curl localhost:2112/metrics

prometheus:
	xdg-open http://localhost:9090

grafana:
	xdg-open http://localhost:3000

app:
	go run main.go

start:
	docker compose up -d
	go run main.go

dev:
	docker compose up -d &
	sleep 3
	go run main.go

status:
	docker ps

logs:
	docker compose logs -f

clean:
	docker compose down -v
	rm -rf _Repos

restart:
	docker compose down
	docker compose up -d &
	sleep 3
	go run main.go