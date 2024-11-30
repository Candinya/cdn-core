.PHONY: genoapi
genoapi:
	mkdir -p app/server/gen/oapi/
	mkdir -p app/worker/gen/oapi/
	go get github.com/oapi-codegen/oapi-codegen/v2
	#go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen -package oapi -generate=types,client,server,spec,skip-prune -o $(OAPI_TARGET)$(OAPI_TARGET_FILENAME) $(OAPI_SPEC)
	go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen -package oapi -generate=types,server,spec,skip-prune -o app/server/gen/oapi/admin.go  spec/admin-api.yml
	go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen -package oapi -generate=types,server,spec,skip-prune -o app/server/gen/oapi/worker.go spec/worker-api.yml
	go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen -package oapi -generate=types,client,skip-prune      -o app/worker/gen/oapi/worker.go spec/worker-api.yml
	go mod tidy
