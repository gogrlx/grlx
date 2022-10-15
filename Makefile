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

sprout: pkg/sprout/*.go
ifeq ($(GOOS),)
	@printf "OS not specified, defaulting to: \e[33m$(UNAME)\e[39m\n"
else
	@printf "OS specified: \e[33m$$(echo $$GOOS | sed 's/.*/\u&/' )\e[39m\n"
endif
	@echo "Building..."
	@if [ ! -n  "$$BITBUCKET_BUILD_NUMBER" ]; then export BITBUCKET_BUILD_NUMBER=$(TYPE); fi;\
	export GOARCH=amd64; \
	export BITBUCKET_BUILD_NUMBER=$(TYPE);\
	export CGO_ENABLED=0;\
	export GitCommit=`git rev-parse HEAD | cut -c -7`;\
	export BuildTime=`date -u +%Y%m%d.%H%M%S`;\
	export Authors=`git log --format='%aN' | sort -u | sed "s@root@@"  | tr '\n' ';' | sed "s@;;@;@g" | sed "s@;@; @g" | sed "s@\(.*      \); @\1@" | sed "s@[[:blank:]]@SpAcE@g"`;\
	export GitTag=$$(TAG=`git tag --contains $$(git rev-parse HEAD) | sort -R | tr '\n' ' '`; if [ "$$(printf "$$TAG")" ]; then printf "$$TAG"; else printf "undefined"; fi);\
	go build -ldflags "-X main.BuildNo=$$BITBUCKET_BUILD_NUMBER -X main.GitCommit=$$GitCommit -X main.Tag=$$GitTag -X main.BuildTime=$$BuildTime -X main.Authors=$$Authors" -o "bin/sprout" ./pkg/sprout/*.go
	@printf "\e[32mSuccess!\e[39m\n"


grlx: pkg/grlx/*.go
ifeq ($(GOOS),)
	@printf "OS not specified, defaulting to: \e[33m$(UNAME)\e[39m\n"
else
	@printf "OS specified: \e[33m$$(echo $$GOOS | sed 's/.*/\u&/' )\e[39m\n"
endif
	@echo "Building..."
	@if [ ! -n  "$$BITBUCKET_BUILD_NUMBER" ]; then export BITBUCKET_BUILD_NUMBER=$(TYPE); fi;\
	export GOARCH=amd64; \
	export BITBUCKET_BUILD_NUMBER=$(TYPE);\
	export CGO_ENABLED=0;\
	export GitCommit=`git rev-parse HEAD | cut -c -7`;\
	export BuildTime=`date -u +%Y%m%d.%H%M%S`;\
	export Authors=`git log --format='%aN' | sort -u | sed "s@root@@"  | tr '\n' ';' | sed "s@;;@;@g" | sed "s@;@; @g" | sed "s@\(.*      \); @\1@" | sed "s@[[:blank:]]@SpAcE@g"`;\
	export GitTag=$$(TAG=`git tag --contains $$(git rev-parse HEAD) | sort -R | tr '\n' ' '`; if [ "$$(printf "$$TAG")" ]; then printf "$$TAG"; else printf "undefined"; fi);\
	go build -ldflags "-X main.BuildNo=$$BITBUCKET_BUILD_NUMBER -X main.GitCommit=$$GitCommit -X main.Tag=$$GitTag -X main.BuildTime=$$BuildTime -X main.Authors=$$Authors" -o "bin/grlx" ./pkg/grlx/main.go
	@printf "\e[32mSuccess!\e[39m\n"


farmer: pkg/farmer/*.go
ifeq ($(GOOS),)
	@printf "OS not specified, defaulting to: \e[33m$(UNAME)\e[39m\n"
else
	@printf "OS specified: \e[33m$$(echo $$GOOS | sed 's/.*/\u&/' )\e[39m\n"
endif
	@echo "Building..."
	@if [ ! -n  "$$BITBUCKET_BUILD_NUMBER" ]; then export BITBUCKET_BUILD_NUMBER=$(TYPE); fi;\
	export GOARCH=amd64; \
	export BITBUCKET_BUILD_NUMBER=$(TYPE);\
	export CGO_ENABLED=0;\
	export GitCommit=`git rev-parse HEAD | cut -c -7`;\
	export BuildTime=`date -u +%Y%m%d.%H%M%S`;\
	export Authors=`git log --format='%aN' | sort -u | sed "s@root@@"  | tr '\n' ';' | sed "s@;;@;@g" | sed "s@;@; @g" | sed "s@\(.*      \); @\1@" | sed "s@[[:blank:]]@SpAcE@g"`;\
	export GitTag=$$(TAG=`git tag --contains $$(git rev-parse HEAD) | sort -R | tr '\n' ' '`; if [ "$$(printf "$$TAG")" ]; then printf "$$TAG"; else printf "undefined"; fi);\
	go build -ldflags "-X main.BuildNo=$$BITBUCKET_BUILD_NUMBER -X main.GitCommit=$$GitCommit -X main.Tag=$$GitTag -X main.BuildTime=$$BuildTime -X main.Authors=$$Authors" -o "bin/farmer" ./pkg/farmer/main.go
	@printf "\e[32mSuccess!\e[39m\n"



clean:  
	@printf "Cleaning up \e[32mmain\e[39m...\n"
	docker-compose down || true
	yes| docker-compose rm || true
	docker rmi grlx/sprout:latest || true
	docker rmi grlx/farmer:latest || true
	rm -f ~/.config/grlx/tls-rootca.pem
	rm -f main bin/grlx bin/farmer bin/sprout

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

run: main
	@echo "Running $(FNAME)..."
	./malware


.PHONY: all
.PHONY: farmer
.PHONY: grlx
.PHONY: sprout
.PHONY: docker
.PHONY: update
