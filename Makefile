# url to a running release-manager
URL=http://localhost:8080
FILE=payload.json

github-webhook:
	curl -H 'Content-Type: application/json' \
	-H 'Accept: */*' \
	-H 'content-type: application/json' \
	-H 'User-Agent: GitHub-Hookshot/d696b2a' \
	-H 'X-GitHub-Delivery: 54b53400-e2de-11ea-8a95-434e7cdb639c' \
	-H 'X-GitHub-Event: pull_request' \
	-H 'X-Hub-Signature: sha1=3631558999d0ba3687b079ee2209dc08293825c6' \
	-d '$(shell cat ${FILE})' \
	$(URL)/webhook/github/bot

prometheus-metrics:
	curl -H 'user_agent: Prometheus/2.20.1' \
	$(URL)/metrics

start:
	go build
	 ./release-manager-bot \
	 --release-manager-auth-token $(HAMCTL_AUTH_TOKEN) \
	 --release-manager-url http://localhost:8081/ \
	 --github-private-key "`cat $(GITHUB_PRIVATE_KEY_PATH)`" \
	 --github-integration-id 75542 \
	 --message-template "`cat $(MESSAGE_TEMPLATE_PATH)`"
