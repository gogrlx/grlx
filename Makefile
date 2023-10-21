UNAME:=$(shell uname|sed 's/.*/\u&/')
OS:=$(shell echo $(GOOS)| sed 's/.*/\u&/')
PKG=$(shell basename $$(pwd))
colon := :
ifeq  ($(BITBUCKET_BUILD_NUMBER),)
TYPE:="Local"
else
TYPE:=$(BITBUCKET_BUILD_NUMBER)
endif


all:  sprout grlx farmer

sprout: cmd/sprout/*.go
ifeq ($(GOOS),)
	@printf "OS not specified, defaulting to: \e[33m$(UNAME)\e[39m\n"
else
	@printf "OS specified: \e[33m$$(echo $$GOOS | sed 's/.*/\u&/' )\e[39m\n"
endif
	@echo "Building..."
	@export GOARCH=amd64; \
	export BITBUCKET_BUILD_NUMBER=$(TYPE);\
	export CGO_ENABLED=0;\
	export GitCommit=`git rev-parse HEAD | cut -c -7`;\
	export GitTag=$$(TAG=`git tag --contains $$(git rev-parse HEAD) | sort -R | tr '\n' ' '`; if [ "$$(printf "$$TAG")" ]; then printf "$$TAG"; else printf "undefined"; fi);\
	go build -ldflags "-X main.GitCommit=$$GitCommit -X main.Tag=$$GitTag" -o "bin/sprout" ./cmd/sprout/*.go
	@printf "\e[32mSuccess!\e[39m\n"


grlx: cmd/grlx/*.go
ifeq ($(GOOS),)
	@printf "OS not specified, defaulting to: \e[33m$(UNAME)\e[39m\n"
else
	@printf "OS specified: \e[33m$$(echo $$GOOS | sed 's/.*/\u&/' )\e[39m\n"
endif
	@echo "Building..."
	@export GOARCH=amd64; \
	export BITBUCKET_BUILD_NUMBER=$(TYPE);\
	export CGO_ENABLED=0;\
	export GitCommit=`git rev-parse HEAD | cut -c -7`;\
	export BuildTime=`date -u +%Y%m%d.%H%M%S`;\
	export GitTag=$$(TAG=`git tag --contains $$(git rev-parse HEAD) | sort -R | tr '\n' ' '`; if [ "$$(printf "$$TAG")" ]; then printf "$$TAG"; else printf "undefined"; fi);\
	go build -ldflags "-X main.GitCommit=$$GitCommit -X main.Tag=$$GitTag" -o "bin/grlx" ./cmd/grlx/main.go
	@printf "\e[32mSuccess!\e[39m\n"


farmer: cmd/farmer/*.go
ifeq ($(GOOS),)
	@printf "OS not specified, defaulting to: \e[33m$(UNAME)\e[39m\n"
else
	@printf "OS specified: \e[33m$$(echo $$GOOS | sed 's/.*/\u&/' )\e[39m\n"
endif
	@echo "Building..."
	@export GOARCH=amd64; \
	export BITBUCKET_BUILD_NUMBER=$(TYPE);\
	export CGO_ENABLED=0;\
	export GitCommit=`git rev-parse HEAD | cut -c -7`;\
	export BuildTime=`date -u +%Y%m%d.%H%M%S`;\
	export GitTag=$$(TAG=`git tag --contains $$(git rev-parse HEAD) | sort -R | tr '\n' ' '`; if [ "$$(printf "$$TAG")" ]; then printf "$$TAG"; else printf "undefined"; fi);\
	go build -ldflags "-X main.GitCommit=$$GitCommit -X main.Tag=$$GitTag" -o "bin/farmer" ./cmd/farmer/main.go
	@printf "\e[32mSuccess!\e[39m\n"

all-arches-farmer: farmer
	@mkdir -p bin/arches
	for arch in amd64 386 arm arm64 ; do \
		export GOOS=linux; \
		export GOARCH=$$arch; \
		export BITBUCKET_BUILD_NUMBER=$(TYPE);\
		export CGO_ENABLED=0;\
		export GitCommit=`git rev-parse HEAD | cut -c -7`;\
		export BuildTime=`date -u +%Y%m%d.%H%M%S`;\
		export GitTag=$$(TAG=`git tag --contains $$(git rev-parse HEAD) | sort -R | tr '\n' ' '`; if [ "$$(printf "$$TAG")" ]; then printf "$$TAG"; else printf "undefined"; fi);\
		go build -ldflags "-X main.GitCommit=$$GitCommit -X main.Tag=$$GitTag" -o "bin/arches/"$$(printf $$GOOS)"/"$$(printf $$GOARCH)"/"$$(printf $$GitTag)"/farmer" ./cmd/farmer/main.go &&\
		printf "\e[32mSuccess!\e[39m\n" ;\
	done

all-arches-sprout: sprout
	@mkdir -p bin/arches
	for arch in amd64 386 arm arm64 ; do \
		export GOOS=linux; \
		export GOARCH=$$arch; \
		export BITBUCKET_BUILD_NUMBER=$(TYPE);\
		export CGO_ENABLED=0;\
		export GitCommit=`git rev-parse HEAD | cut -c -7`;\
		export BuildTime=`date -u +%Y%m%d.%H%M%S`;\
		export GitTag=$$(TAG=`git tag --contains $$(git rev-parse HEAD) | sort -R | tr '\n' ' '`; if [ "$$(printf "$$TAG")" ]; then printf "$$TAG"; else printf "undefined"; fi);\
		go build -ldflags "-X main.GitCommit=$$GitCommit -X main.Tag=$$GitTag" -o "bin/arches/"$$(printf $$GOOS)"/"$$(printf $$GOARCH)"/"$$(printf $$GitTag)"/sprout" ./cmd/sprout/*.go &&\
		printf "\e[32mSuccess!\e[39m\n" ;\
	done

all-arches-grlx: grlx
	@mkdir -p bin/arches
	for arch in amd64 386 arm arm64 ; do \
			export GOOS=linux; \
			export GOARCH=$$arch; \
			export BITBUCKET_BUILD_NUMBER=$(TYPE);\
			export CGO_ENABLED=0;\
			export GitCommit=`git rev-parse HEAD | cut -c -7`;\
			export BuildTime=`date -u +%Y%m%d.%H%M%S`;\
			export GitTag=$$(TAG=`git tag --contains $$(git rev-parse HEAD) | sort -R | tr '\n' ' '`; if [ "$$(printf "$$TAG")" ]; then printf "$$TAG"; else printf "undefined"; fi);\
			go build -ldflags "-X main.GitCommit=$$GitCommit -X main.Tag=$$GitTag" -o "bin/arches/"$$(printf $$GOOS)"/"$$(printf $$GOARCH)"/"$$(printf $$GitTag)"/grlx" ./cmd/grlx/main.go &&\
			printf "\e[32mSuccess!\e[39m\n" ;\
	done
	for arch in amd64 arm64 ; do \
			export GOOS=darwin; \
			export GOARCH=$$arch; \
			export BITBUCKET_BUILD_NUMBER=$(TYPE);\
			export CGO_ENABLED=0;\
			export GitCommit=`git rev-parse HEAD | cut -c -7`;\
			export BuildTime=`date -u +%Y%m%d.%H%M%S`;\
			export GitTag=$$(TAG=`git tag --contains $$(git rev-parse HEAD) | sort -R | tr '\n' ' '`; if [ "$$(printf "$$TAG")" ]; then printf "$$TAG"; else printf "undefined"; fi);\
			go build -ldflags "-X main.GitCommit=$$GitCommit -X main.Tag=$$GitTag" -o "bin/arches/"$$(printf $$GOOS)"/"$$(printf $$GOARCH)"/"$$(printf $$GitTag)"/grlx" ./cmd/grlx/main.go &&\
			printf "\e[32mSuccess!\e[39m\n" ;\
	done



release: all-arches-farmer all-arches-sprout all-arches-grlx
	@printf "\e[32mSuccess!\e[39m\n"



clean:
	@printf "Cleaning up \e[32mmain\e[39m...\n"
	docker-compose down || true
	yes| docker-compose rm || true
	docker rmi grlx/sprout:latest || true
	docker rmi grlx/farmer:latest || true
	rm -f ~/.config/grlx/tls-rootca.pem
	rm -f main bin/grlx bin/farmer bin/sprout
	rm -r bin/arches || true

install: clean all
	mv bin/grlx bin/farmer bin/sprout "$$GOPATH/bin/"

docker:
	docker build -t grlx/farmer . -f docker/farmer.dockerfile
	docker build -t grlx/sprout . -f docker/sprout.dockerfile

dcu:
	docker-compose down || true
	docker-compose rm
	rm -f ~/.config/grlx/tls-rootca.pem
	docker-compose up

test: clean 
	docker-compose build
	docker-compose up -d
	@printf "\e[31mNo tests defined!\e[39m\n"
	docker compose down
	@exit 1


.PHONY: all
.PHONY: clean
.PHONY: docker
.PHONY: install
.PHONY: test
.PHONY: update
.PHONY: release
