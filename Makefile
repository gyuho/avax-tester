# run from repository root

build:
	go build -o bin/avax-tester cmd/avax-tester/main.go

clean:
	rm -f bin/avax-tester

fmt:
	./fmt.sh

update:
	./vend.sh

install:
	go install -v ./cmd/avax-tester
