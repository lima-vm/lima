# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

load "../helpers/load"

# The jandubois/jandubois GitHub repo has been especially constructed to test
# various features of the github URL scheme:
#
# * repo defaults to org when not specified
# * filename defaults to .lima.yaml when only a path is specified
# * .yaml default extension
# * .lima.yaml files may be treated as symlinks
# * default branch lookup when not specified
#
# The repo files are:
#
# ├── .lima.yaml -> templates/demo.yaml
# ├── docs
# │   └── .lima.yaml -> ../templates/demo.yaml
# └── templates
#     └── demo.yaml
#
# Both the `main` branch and the `v0.0.0` tag have this layout.

# All these URLs should redirect to the same template URL (either on "main" or at "v0.0.0"):
# "https://raw.githubusercontent.com/jandubois/jandubois/${tag}/templates/demo.yaml"
URLS=(
	github:jandubois/jandubois/templates/demo.yaml@main
	github:jandubois/jandubois/templates/demo.yaml
	github:jandubois/jandubois/templates/demo
	github:jandubois/jandubois/.lima.yaml
	github:jandubois/jandubois/@v0.0.0
	github:jandubois/jandubois
	github:jandubois//templates/demo.yaml@main
	github:jandubois//templates/demo.yaml
	github:jandubois//templates/demo
	github:jandubois//.lima.yaml
	github:jandubois//@v0.0.0
	github:jandubois//
	github:jandubois/
	github:jandubois@v0.0.0
	github:jandubois
	github:jandubois/jandubois/docs/.lima.yaml@main
	github:jandubois/jandubois/docs/.lima.yaml
	github:jandubois/jandubois/docs/.lima
	github:jandubois/jandubois/docs/
	github:jandubois//docs/.lima.yaml@v0.0.0
	github:jandubois//docs/.lima.yaml
	github:jandubois//docs/.lima
	github:jandubois//docs/@v0.0.0
	github:jandubois//docs/
)

url() {
	run_e "$1" limactl template url "$2"
}

test_jandubois_url() {
	local url=$1
	local tag="main"
	if [[ $url == *v0.0.0* ]]; then
		tag="v0.0.0"
	fi

	url -0 "$url"
	assert_output "https://raw.githubusercontent.com/jandubois/jandubois/${tag}/templates/demo.yaml"
}

# Dynamically register a @test for each URL in the list
for url in "${URLS[@]}"; do
	bats_test_function --description "$url" -- test_jandubois_url "$url"
done

@test '.lima.yaml is retained when it is not a symlink' {
	url -0 'github:jandubois//test/'
	assert_output 'https://raw.githubusercontent.com/jandubois/jandubois/main/test/.lima.yaml'
}

@test 'hidden files without an extension get a .yaml extension' {
	url -0 'github:jandubois//test/.hidden'
	assert_output 'https://raw.githubusercontent.com/jandubois/jandubois/main/test/.hidden.yaml'
}

@test 'files that have an extension do not get a .yaml extension' {
	url -0 'github:jandubois//test/.script.sh'
	assert_output 'https://raw.githubusercontent.com/jandubois/jandubois/main/test/.script.sh'
}

@test 'github: URLs are EXPERIMENTAL' {
	url -0 'github:jandubois'
	assert_stderr --regexp 'warning.+GitHub locator .* replaced with .* EXPERIMENTAL'
}

@test 'Empty github: url returns an error' {
	url -1 'github:'
	assert_fatal 'github: URL must contain at least an ORG, got ""'
}

@test 'Missing org returns an error' {
	url -1 'github:/jandubois'
	assert_fatal 'github: URL must contain at least an ORG, got ""'
}
