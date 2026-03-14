# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

load "../helpers/load"

# The jandubois/jandubois GitHub repo has been especially constructed to test
# various features of the github URL scheme:
#
#   * repo defaults to org when not specified
#   * filename defaults to .lima.yaml when only a path is specified
#   * .yaml default extension
#   * .lima.yaml files may be treated as symlinks
#   * default branch lookup when not specified
#   * github:ORG// repos can redirect to another github:ORG URL in the same ORG
#
# The jandubois/jandubois repo files are:
#
#   ├── .lima.yaml         -> templates/demo.yaml
#   ├── back
#   │   └── .lima.yaml     -> github:jandubois//loop/
#   ├── docs
#   │   └── .lima.yaml     -> ../templates/demo.yaml
#   ├── example.yaml       -> templates/demo.yaml
#   ├── invalid
#   │   ├── org
#   │   │   └── .lima.yaml -> github:lima-vm
#   │   └── tag
#   │       └── .lima.yaml -> github:jandubois//@v0.0.0
#   ├── loop
#   │   └── .lima.yaml     -> github:jandubois//back/
#   ├── redirect
#   │   └── .lima.yaml     -> github:jandubois/lima/templates/default
#   ├── templates
#   │   └── demo.yaml      "base: template:default"
#   └── yaml
#       └── .lima.yaml     "{}"
#
# Both the `main` branch and the `v0.0.0` tag have this layout.
#
# All these URLs should redirect to the same template URL (either on "main" or at "v0.0.0"):
# "https://raw.githubusercontent.com/jandubois/jandubois/${tag}/templates/demo.yaml"
#
# Additional tests rely on jandubois/lima existing and containing the v1.2.1 tag.

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
	github:jandubois/jandubois/example.yaml
	github:jandubois/jandubois/example@main
	github:jandubois//example.yaml@v0.0.0
	github:jandubois//example
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
	run_e "$1" limactl --debug template url "$2"
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

@test '.lima.yaml is retained when it exits and is not a symlink' {
	url -0 'github:jandubois//yaml/'
	assert_output 'https://raw.githubusercontent.com/jandubois/jandubois/main/yaml/.lima.yaml'
}

@test 'non-existing .lima.yaml returns an error' {
	url -1 'github:jandubois//missing/'
	assert_fatal 'file "https://raw.githubusercontent.com/jandubois/jandubois/main/missing/.lima.yaml" not found or inaccessible: status 404'
}

@test 'hidden files without an extension get a .yaml extension' {
	url -1 'github:jandubois//test/.hidden'
	assert_fatal 'file "https://raw.githubusercontent.com/jandubois/jandubois/main/test/.hidden.yaml" not found or inaccessible: status 404'
}

@test 'files that have an extension do not get a .yaml extension' {
	# This command doesn't fail because only *.yaml files are checked for redirects/symlinks, and therefore fail right away if they don't exist.
	url -0 'github:jandubois//test/.script.sh'
	assert_output 'https://raw.githubusercontent.com/jandubois/jandubois/main/test/.script.sh'
}

@test 'github: URLs are EXPERIMENTAL' {
	url -0 'github:jandubois'
	assert_warning "The github: scheme is still EXPERIMENTAL"
}

# Invalid URLs
@test 'empty github: url returns an error' {
	url -1 'github:'
	assert_fatal 'github: URL must contain at least an ORG, got ""'
}

@test 'missing org returns an error' {
	url -1 'github:/jandubois'
	assert_fatal 'github: URL must contain at least an ORG, got ""'
}

# github: redirects in github:ORG// repos
@test 'org redirects can point to different repo and may switch the branch name' {
	url -0 'github:jandubois//redirect/'
	# Note that the default branch in jandubois/jandubois is main, but in jandubois/lima it is master
	assert_debug 'Locator "github:jandubois//redirect/" replaced with "github:jandubois/lima/templates/default"'
	assert_debug 'Locator "github:jandubois/lima/templates/default" replaced with "https://raw.githubusercontent.com/jandubois/lima/master/templates/default.yaml"'
	assert_output 'https://raw.githubusercontent.com/jandubois/lima/master/templates/default.yaml'
}

@test 'org redirects propagate an explicit branch/tag to the other repo' {
	url -0 'github:jandubois//redirect/@v1.2.1'
	assert_debug 'Locator "github:jandubois//redirect/@v1.2.1" replaced with "github:jandubois/lima/templates/default@v1.2.1"'
	assert_debug 'Locator "github:jandubois/lima/templates/default@v1.2.1" replaced with "https://raw.githubusercontent.com/jandubois/lima/v1.2.1/templates/default.yaml"'
	assert_output 'https://raw.githubusercontent.com/jandubois/lima/v1.2.1/templates/default.yaml'
}

@test 'org redirects cannot point to another org' {
	url -1 'github:jandubois//invalid/org/'
	assert_fatal 'redirect "github:lima-vm" is not a "github:jandubois" URL…'
}

@test 'org redirects with branch cannot point to another org' {
	url -1 'github:jandubois//invalid/org/@main'
	assert_fatal 'redirect "github:lima-vm" is not a "github:jandubois" URL…'
}

@test 'org redirects cannot include a branch or tag' {
	url -1 'github:jandubois//invalid/tag/'
	assert_fatal 'redirect "github:jandubois//@v0.0.0" must not include a branch/tag/sha…'
}

@test 'org redirects with tag cannot include a branch or tag' {
	url -1 'github:jandubois//invalid/tag/@v0.0.0'
	assert_fatal 'redirect "github:jandubois//@v0.0.0" must not include a branch/tag/sha…'
}

@test 'org redirects must not create circular redirects' {
	url -1 'github:jandubois//loop/'
	assert_fatal 'custom locator "github:jandubois//loop/" has a redirect loop'
}

@test 'org redirects with branch must not create circular redirects' {
	url -1 'github:jandubois//back/@main'
	assert_fatal 'custom locator "github:jandubois//back/@main" has a redirect loop'
}
