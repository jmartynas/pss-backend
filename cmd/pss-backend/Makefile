.PHONY: run
run: build
	env $(shell cat .env) ./pss-backend

.PHONY: build
build:
	go build -o pss-backend
