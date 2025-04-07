all: clean
	for f in $(wildcard *.go); do echo $$f; go build -ldflags="-s -w" $$f; done

clean:
	rm -f cat chmod chown cp echo rm touch