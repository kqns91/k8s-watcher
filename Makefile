.PHONY: build run test docker-build docker-push deploy clean lint lint-fix

# Variables
BINARY_NAME=kube-watcher
DOCKER_IMAGE=kube-watcher
DOCKER_TAG=latest
NAMESPACE=default

# Build the binary
build:
	go build -o $(BINARY_NAME) ./cmd

# Run locally
run:
	go run ./cmd/main.go -config config/config.yaml

# Run tests
test:
	go test -v ./...

# Build Docker image
docker-build:
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .

# Push Docker image
docker-push: docker-build
	docker push $(DOCKER_IMAGE):$(DOCKER_TAG)

# Deploy to Kubernetes
deploy:
	kubectl apply -f deployments/rbac.yaml
	kubectl apply -f deployments/secret.yaml
	kubectl apply -f deployments/configmap.yaml
	kubectl apply -f deployments/deployment.yaml

# Undeploy from Kubernetes
undeploy:
	kubectl delete -f deployments/deployment.yaml --ignore-not-found
	kubectl delete -f deployments/configmap.yaml --ignore-not-found
	kubectl delete -f deployments/secret.yaml --ignore-not-found
	kubectl delete -f deployments/rbac.yaml --ignore-not-found

# Clean build artifacts
clean:
	rm -f $(BINARY_NAME)

# Download dependencies
deps:
	go mod download
	go mod tidy

# Format code
fmt:
	go fmt ./...

# Lint code
lint:
	@which golangci-lint > /dev/null || (echo "golangci-lint not installed. Run: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest" && exit 1)
	golangci-lint run

# Lint and fix code
lint-fix:
	@which golangci-lint > /dev/null || (echo "golangci-lint not installed. Run: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest" && exit 1)
	golangci-lint run --fix

# Show logs
logs:
	kubectl logs -f -l app=kube-watcher -n $(NAMESPACE)

# Restart deployment
restart:
	kubectl rollout restart deployment/kube-watcher -n $(NAMESPACE)
