tests:
	go test ./... -v

unit-test:
	go test ./internal/*/domain ./internal/*/application -v -cover

coverage:
	go test ./... -coverprofile=coverage.out
	go tool cover -func=coverage.out

coverage-html:
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage -o coverage.html
	@echo "👉 Abre coverage.html en tu navegador"

clean:
	rm -f coverage.out coverage.html

build-proto:
	protoc --go_out=. --go-grpc_out=. proto/task.proto