DOKKU_APP ?= calli
DOKKU_DEPLOY_URL ?= dokku@dev.lookingfora.name

dokku-build:
	docker build \
		-t calli-dokku:latest \
		-f misc/dokku/Dockerfile \
		.

dokku-deploy:
	git push --atomic $(DOKKU_DEPLOY_URL):$(DOKKU_APP) $(shell git rev-parse HEAD):refs/heads/master --force