# url to a running release-manager
URL=https://api.dev.lunarway.com/release-manager-bot
FILE=payload.json
PAYLOAD=`cat $(FILE)`

github-webhook:
	curl -H 'X-GitHub-Event: pull_request' \
	-H 'Content-Type: application/json' \
	-d '$(shell cat ${FILE})' \
	$(URL)/webhook/github/bot
