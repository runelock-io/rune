VERSION := v1.0.0-RC
LDFLAGS := -ldflags="-s -w -X rune/internal/cli.Version=$(VERSION)"
BIN     := bin

$(BIN):
	mkdir -p $(BIN)

build-ui:
	cd ui && npm install && npm run build

cli-build: $(BIN)
	go build $(LDFLAGS) -o $(BIN)/rune ./cmd/cli

operator-build: $(BIN)
	go build -o $(BIN)/rune-operator ./cmd/operator

operator-image: operator-build
	docker build -f Dockerfile.operator -t rune-operator:dev .

server-build: build-ui $(BIN)
	go build $(LDFLAGS) -o $(BIN)/rune-server ./cmd/server

cli-install: cli-build
	sudo cp $(BIN)/rune /usr/local/bin/rune

server-install: server-build
	sudo cp $(BIN)/rune-server /usr/local/bin/rune-server

install: cli-install server-install

clean:
	rm -rf $(BIN)
