.PHONY: build docker run

IMAGE_NAME = sheet-gridly-integration
CONTAINER_NAME = sheet-gridly


docker:
	docker build -t $(IMAGE_NAME) .

run: 
	docker run -d --name $(CONTAINER_NAME) \
	--env-file .env \
	-v ./client_secret.json:/app_config/client_secret.json \
	$(IMAGE_NAME)
