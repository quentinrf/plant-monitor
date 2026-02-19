SERVICES := light-service plant-service dashboard-service

.PHONY: proto build test docker-build up certs k8s-deploy k8s-delete k8s-status k8s-certs

## proto: Regenerate all protobuf/gRPC code via buf
proto:
	cd services/light-service && buf generate
	cd services/plant-service && buf generate

## build: Build all service binaries into bin/
build:
	@for svc in $(SERVICES); do \
		echo "Building $$svc..."; \
		go build -o bin/$$svc ./services/$$svc/cmd/server; \
	done

## test: Run tests for all services
test:
	@for svc in $(SERVICES); do \
		echo "Testing $$svc..."; \
		cd services/$$svc && go test ./... && cd ../..; \
	done

## docker-build: Build Docker images for all services
docker-build:
	@for svc in $(SERVICES); do \
		echo "Building image plant-monitor/$$svc..."; \
		docker build -t plant-monitor/$$svc:latest ./services/$$svc; \
	done

## up: Start all services with docker compose
up:
	docker compose up --build

## certs: Generate mTLS certificates (requires openssl)
certs:
	bash scripts/gen-certs.sh

## k8s-certs: Create TLS secret in Kubernetes from generated certs
k8s-certs:
	kubectl create secret generic tls-certs -n plant-monitor \
		--from-file=ca.crt=certs/ca.crt \
		--from-file=light-service.crt=certs/light-service.crt \
		--from-file=light-service.key=certs/light-service.key \
		--from-file=plant-service.crt=certs/plant-service.crt \
		--from-file=plant-service.key=certs/plant-service.key \
		--from-file=dashboard-service.crt=certs/dashboard-service.crt \
		--from-file=dashboard-service.key=certs/dashboard-service.key \
		--dry-run=client -o yaml > k8s/secrets/tls-certs.yaml

## k8s-deploy: Apply all Kubernetes manifests
k8s-deploy:
	kubectl apply -f k8s/namespace.yaml
	kubectl apply -f k8s/secrets/tls-certs.yaml
	kubectl apply -f k8s/light-service/
	kubectl apply -f k8s/plant-service/
	kubectl apply -f k8s/dashboard-service/

## k8s-delete: Tear down the plant-monitor namespace
k8s-delete:
	kubectl delete namespace plant-monitor

## k8s-status: Show pod and service status in the plant-monitor namespace
k8s-status:
	kubectl get pods -n plant-monitor
	kubectl get services -n plant-monitor
