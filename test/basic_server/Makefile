REGISTRY = 807158446417.dkr.ecr.us-west-2.amazonaws.com
VERSION = 1

docker:
	docker build . -t "$(REGISTRY)/basic-server:$(VERSION)"
	docker push "$(REGISTRY)/basic-server:$(VERSION)"

run:
	go build .
	./basic_server

run-docker:
	docker run -p 8080:8080 -t "$(REGISTRY)/basic-server:$(VERSION)"
