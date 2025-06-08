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
	go build -ldflags "-X main.GitCommit=$$GitCommit -X main.Tag=$$GitTag" -o "bin/grlx-sprout" ./cmd/sprout/*.go
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
	go build -ldflags "-X main.GitCommit=$$GitCommit -X main.Tag=$$GitTag" -o "bin/grlx-farmer" ./cmd/farmer/main.go
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
		go build -ldflags "-X main.GitCommit=$$GitCommit -X main.Tag=$$GitTag" -o "bin/arches/"$$(printf $$GOOS)"/"$$(printf $$GOARCH)"/"$$(printf $$GitTag)"/grlx-farmer" ./cmd/farmer/main.go &&\
		printf "\e[32mSuccess!\e[39m\n" ;\
		mkdir -p bin/arches/"$$(printf $$GOOS)"/"$$(printf $$GOARCH)"/latest ;\
		cp bin/arches/"$$(printf $$GOOS)"/"$$(printf $$GOARCH)"/"$$(printf $$GitTag)"/farmer bin/arches/"$$(printf $$GOOS)"/"$$(printf $$GOARCH)"/latest/grlx-farmer ;\
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
		go build -ldflags "-X main.GitCommit=$$GitCommit -X main.Tag=$$GitTag" -o "bin/arches/"$$(printf $$GOOS)"/"$$(printf $$GOARCH)"/"$$(printf $$GitTag)"/grlx-sprout" ./cmd/sprout/*.go &&\
		printf "\e[32mSuccess!\e[39m\n" ;\
		mkdir -p bin/arches/"$$(printf $$GOOS)"/"$$(printf $$GOARCH)"/latest ;\
		cp bin/arches/"$$(printf $$GOOS)"/"$$(printf $$GOARCH)"/"$$(printf $$GitTag)"/sprout bin/arches/"$$(printf $$GOOS)"/"$$(printf $$GOARCH)"/latest/grlx-sprout ;\
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
			mkdir -p bin/arches/"$$(printf $$GOOS)"/"$$(printf $$GOARCH)"/latest ;\
			cp bin/arches/"$$(printf $$GOOS)"/"$$(printf $$GOARCH)"/"$$(printf $$GitTag)"/grlx bin/arches/"$$(printf $$GOOS)"/"$$(printf $$GOARCH)"/latest/grlx ;\
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
			mkdir -p bin/arches/"$$(printf $$GOOS)"/"$$(printf $$GOARCH)"/latest ;\
			cp bin/arches/"$$(printf $$GOOS)"/"$$(printf $$GOARCH)"/"$$(printf $$GitTag)"/grlx bin/arches/"$$(printf $$GOOS)"/"$$(printf $$GOARCH)"/latest/grlx ;\
	done

github: all-arches-farmer all-arches-sprout all-arches-grlx
	@printf "Creating GitHub release...\n"
	mkdir -p bin/github
	for arch in amd64 386 arm arm64 ; do \
		export GitTag=$$(TAG=`git tag --contains $$(git rev-parse HEAD) | sort -R | tr '\n' ' '`; if [ "$$(printf "$$TAG")" ]; then printf "$$TAG"; else printf "undefined"; fi);\
		cp bin/arches/linux/$$arch/$$(printf $$GitTag)/farmer bin/github/grlx-farmer-$$(printf $$GitTag)-linux-$$(printf $$arch);\
		tar -czf bin/github/grlx-farmer-$$(printf $$GitTag)-linux-$$(printf $$arch).tar.gz \
	      -C bin/arches/linux/$$arch/$$(printf $$GitTag) grlx-farmer;\
	done
	for arch in amd64 386 arm arm64 ; do \
		export GitTag=$$(TAG=`git tag --contains $$(git rev-parse HEAD) | sort -R | tr '\n' ' '`; if [ "$$(printf "$$TAG")" ]; then printf "$$TAG"; else printf "undefined"; fi);\
		cp bin/arches/linux/$$arch/$$(printf $$GitTag)/sprout bin/github/grlx-sprout-$$(printf $$GitTag)-linux-$$(printf $$arch);\
		tar -czf bin/github/grlx-sprout-$$(printf $$GitTag)-linux-$$(printf $$arch).tar.gz \
			-C bin/arches/linux/$$arch/$$(printf $$GitTag) grlx-sprout;\
	done
	for arch in amd64 arm64 ; do \
		export GitTag=$$(TAG=`git tag --contains $$(git rev-parse HEAD) | sort -R | tr '\n' ' '`; if [ "$$(printf "$$TAG")" ]; then printf "$$TAG"; else printf "undefined"; fi);\
		cp bin/arches/darwin/$$arch/$$(printf $$GitTag)/grlx bin/github/grlx-$$(printf $$GitTag)-darwin-$$(printf $$arch);\
		tar -czf bin/github/grlx-$$(printf $$GitTag)-darwin-$$(printf $$arch).tar.gz \
			-C bin/arches/darwin/$$arch/$$(printf $$GitTag) grlx;\
	done
	for arch in amd64 386 arm arm64 ; do \
		export GitTag=$$(TAG=`git tag --contains $$(git rev-parse HEAD) | sort -R | tr '\n' ' '`; if [ "$$(printf "$$TAG")" ]; then printf "$$TAG"; else printf "undefined"; fi);\
		cp bin/arches/linux/$$arch/$$(printf $$GitTag)/grlx bin/github/grlx-$$(printf $$GitTag)-linux-$$(printf $$arch);\
		tar -czf bin/github/grlx-$$(printf $$GitTag)-linux-$$(printf $$arch).tar.gz \
			-C bin/arches/linux/$$arch/$$(printf $$GitTag) grlx;\
	done
	
release: all-arches-farmer all-arches-sprout all-arches-grlx github
	@printf "\e[32mSuccess!\e[39m\n"



clean:
	@printf "Cleaning up \e[32mmain\e[39m...\n"
	docker-compose down || true
	yes| docker-compose rm || true
	docker rmi grlx/sprout:latest || true
	docker rmi grlx/farmer:latest || true
	rm -f ~/.config/grlx/tls-rootca.pem
	rm -f main bin/grlx bin/farmer bin/sprout
	rm -r bin/arches bin/github || true

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
