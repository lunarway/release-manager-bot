# url to a running release-manager
URL=localhost:8080
FILE=payload.json
PAYLOAD=`cat $(FILE)`

github-webhook:
	curl -H 'X-GitHub-Event: pull_request' \
	-H 'Content-Type: application/json' \
	-d '$(shell cat ${FILE})' \
	$(URL)/webhook/github
