set -euo pipefail

source "${BASH_SOURCE%/*}/gen_maxVersions_genconfig.bash"

function doBranchCheck {
	if [ "${CI:-}" != "true" ]
	then
		return
	fi
	# we are on CI
	if [ "${TRAVIS_PULL_REQUEST:-}" == "" ] || [[ "$(curl -s -u $GH_USER:$GH_TOKEN https://api.github.com/repos/govim/govim/pulls/$TRAVIS_PULL_REQUEST | jq -r .title)" != \[WIP\]* ]]
	then
		return
	fi
	# We have a WIP pull request
	if [ "$(eval "echo \${CI_MUST_RUN_${TRAVIS_PULL_REQUEST}:-}")" == "true" ]
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
	exit 0
}

