.PHONY: genoapi
genoapi:
	mkdir -p app/server/gen/oapi/admin/ app/server/gen/oapi/worker/
	mkdir -p app/worker/gen/oapi/worker/
	go get github.com/oapi-codegen/oapi-codegen/v2
	go get github.com/oapi-codegen/oapi-codegen/v2/pkg/util
	go get github.com/oapi-codegen/oapi-codegen/v2/pkg/codegen
	go get github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen
	#go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen -package oapi -generate=types,client,server,spec,skip-prune -o $(OAPI_TARGET)$(OAPI_TARGET_FILENAME) $(OAPI_SPEC)
	go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen -package admin  -generate=types,server,spec,skip-prune -o app/server/gen/oapi/admin/oapi.go  spec/admin-api.yml
	go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen -package worker -generate=types,server,spec,skip-prune -o app/server/gen/oapi/worker/oapi.go spec/worker-api.yml
	#go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen -package worker -generate=types,client,skip-prune      -o app/worker/gen/oapi/worker/oapi.go spec/worker-api.yml
	go mod tidy
