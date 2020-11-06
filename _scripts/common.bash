set -euo pipefail

source "${BASH_SOURCE%/*}/gen_maxVersions_genconfig.bash"

if [ "${CI:-}" == "true" ] && [ "${GITHUB_EVENT_NAME:-}" == "pull_request" ]
then
	export GITHUB_PR_NUMBER=$(echo ${GITHUB_REF:-} | grep refs/pull | sed -e 's+/+ +g' | awk '{print $3}')
	if [ "$GITHUB_PR_NUMBER" == "" ]
	then
		echo "Failed to get PR number from GITHUB_REF=${GITHUB_REF:-}"
		exit 1
	fi
fi

function doBranchCheck {
	if [ "${CI:-}" != "true" ]
	then
		return
	fi
	# we are on CI
	if [ "${GITHUB_EVENT_NAME:-}" != "pull_request" ]
	then
		return
	fi
	# we are building a pull request
	if [[ "$(curl -s -u $GH_USER:$GH_TOKEN https://api.github.com/repos/govim/govim/pulls/$GITHUB_PR_NUMBER | jq -r .title)" != \[WIP\]* ]]
	then
		return
	fi
	# We have a WIP pull request
	if [ "$(eval "echo \${CI_MUST_RUN_${GITHUB_PR_NUMBER}:-}")" == "true" ]
	then
		# we must run all builds in the matrix for this PR
		return
	fi
	if [ "$GO_VERSION" == "$MAX_GO_VERSION" ] && [ "$VIM_VERSION" == "$(eval "echo \$MAX_${VIM_FLAVOR^^}_VERSION")" ]
	then
		# We want to run this build
		return
	fi
	echo "Skipping build for ${VIM_FLAVOR}_${VIM_VERSION}_${GO_VERSION}"
	echo "CI_SKIP_JOB=true" >> $GITHUB_ENV
	exit 0
}
