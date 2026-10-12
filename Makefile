SHELL := /bin/sh

IMAGE ?= ghcr.io/aobaiwaki123/lumipage
TAG ?= dev

.PHONY: test verify build run docker-build docker-push k8s-render k8s-apply

test:
	go test ./...

verify:
	./scripts/verify-all.sh

build:
	go build -trimpath -o /tmp/lumipage ./cmd/lumipage

run:
	go run ./cmd/lumipage

docker-build:
	docker build --tag $(IMAGE):$(TAG) .

docker-push:
	docker push $(IMAGE):$(TAG)

k8s-render:
	kubectl kustomize k8s/manifests

k8s-apply:
	kubectl apply -k k8s/manifests
