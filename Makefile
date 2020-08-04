# url to a running release-manager
URL=localhost:8080

github-webhook:
	curl -H 'X-GitHub-Event: push' \
	-H ''\
	'\
	-d '{ \
		"ref": "refs/heads/master", \
		"head_commit": { \
			"id": "sha", \
			"message": "[product] artifact master-1234ds13g3-12s46g356g by Foo Bar\nArtifact-created-by: Foo Bar <test@lunar.app>" \
		} \
	}' \
	$(URL)/webhook/github