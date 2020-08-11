# url to a running release-manager
URL=http://localhost:8080
FILE=payload.json

github-webhook:
	curl -H 'X-GitHub-Event: pull_request' \
	-H 'Content-Type: application/json' \
	-d '$(shell cat ${FILE})' \
	$(URL)/webhook/github/bot



start:
	go build
	 ./release-manager-bot \
	 --release-manager-auth-token $(HAMCTL_AUTH_TOKEN) \
	 --release-manager-url http://localhost:8081/ \
	 --github-private-key "`cat $(GITHUB_PRIVATE_KEY_PATH)`" \
	 --github-integration-id 75542
