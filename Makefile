# GoTinyMUSH Development Makefile
#
# Quick start after cloning:
#   make dev
#
# This starts the server in fresh mode (reimports from flatfile every restart)
# Connect with: telnet localhost 6886
# Login with:   connect Wizard potrzebie

BINARY    = gotinymush
VERSION   = 0.2.0
CONF      = data/crystal.yaml
DB        = data/crystal.FLAT.LATEST
BOLT      = data/game.bolt
TEXTDIR   = data/text
ALIASCONF = data/goTinyAlias.conf
COMSYSDB  = data/mod_comsys.db
PORT      = 6886
LDFLAGS   = -X github.com/crystal-mush/gotinymush/pkg/server.Version=$(VERSION)

.PHONY: build dev run fresh clean test vet

# Build the server binary
build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/server

# Development mode: fresh reimport every restart (recommended during dev)
dev: build
	./$(BINARY) -conf $(CONF) -db $(DB) -bolt $(BOLT) -textdir $(TEXTDIR) \
		-aliasconf $(ALIASCONF) -comsysdb $(COMSYSDB) -port $(PORT) -fresh

# Normal run: persists bolt DB between restarts
run: build
	./$(BINARY) -conf $(CONF) -db $(DB) -bolt $(BOLT) -textdir $(TEXTDIR) \
		-aliasconf $(ALIASCONF) -comsysdb $(COMSYSDB) -port $(PORT)

# Same as dev (explicit name)
fresh: dev

# Remove built binary and bolt database
clean:
	rm -f $(BINARY) $(BINARY).exe $(BOLT)

# Run tests
test:
	go test ./...

# Run go vet
vet:
	go vet ./...
