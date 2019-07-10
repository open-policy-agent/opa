.PHONY: realclean cover viewcover

realclean:
	rm coverage.out

cover:
	go test -v -coverpkg=./... -coverprofile=coverage.out ./...

viewcover:
	go tool cover -html=coverage.out
