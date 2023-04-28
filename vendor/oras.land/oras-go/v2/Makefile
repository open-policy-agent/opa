.PHONY: test
test: vendor check-encoding
	go test -race -v -coverprofile=coverage.txt -covermode=atomic ./...

.PHONY: covhtml
covhtml:
	open .cover/coverage.html

.PHONY: clean
clean:
	git status --ignored --short | grep '^!! ' | sed 's/!! //' | xargs rm -rf

.PHONY: check-encoding
check-encoding:
	! find . -not -path "./vendor/*" -name "*.go" -type f -exec file "{}" ";" | grep CRLF
	! find scripts -name "*.sh" -type f -exec file "{}" ";" | grep CRLF

.PHONY: fix-encoding
fix-encoding:
	find . -not -path "./vendor/*" -name "*.go" -type f -exec sed -i -e "s/\r//g" {} +
	find scripts -name "*.sh" -type f -exec sed -i -e "s/\r//g" {} +

.PHONY: vendor
vendor:
	go mod vendor
