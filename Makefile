# url to a running release-manager
URL=localhost:8080
FILE=payload.json
PAYLOAD=`cat $(FILE)`

github-webhook:
	curl -H 'X-GitHub-Event: pull_request' \
	-H 'Content-Type: application/json' \
	-d '$(shell cat ${FILE})' \
	$(URL)/webhook/github



github-webhook-jacob:
	curl -H 'POST /payload HTTP/1.1\
	Host: localhost:4567\
	X-GitHub-Delivery: 72d3162e-cc78-11e3-81ab-4c9367dc0958\
	X-Hub-Signature: sha1=7d38cdd689735b008b3c702edd92eea23791c5f6\
	User-Agent: GitHub-Hookshot/044aadd\
	Content-Type: application/json\
	Content-Length: 6615\
	X-GitHub-Event: issues'\
	-d '${cat payload.json}'