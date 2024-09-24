#!/bin/sh

# This script is used to wrap the compiler and linker commands in the build
# process. It captures the output of the command and logs it to a file.
# The script's primary purpose is codesigning the output of the linker command
# with the entitlements file if it exists.
# If the OS is macOS, the result of the command is 0, the entitlements file
# exists, and codesign is available, sign the output of the linker command with
# the entitlements file.
#
# Usage:
#   go build -toolexec hack/toolexec-to-codesign.sh

repository_root="$(dirname "$(dirname "$0")")"
logfile="${repository_root}/.toolexec-to-codesign.log"

echo $$: cmd: "$@" >>"${logfile}"

output="$("$@")"
result=$?

echo $$: output: "${output}" >>"${logfile}"

entitlements="${repository_root}/vz.entitlements"

# If the OS is macOS, the result of the command is 0, the entitlements file
# exists, and codesign is available, sign the output of the linker command.
if OS=$(uname -s) && [ "${OS}" = "Darwin" ] && [ "${result}" -eq 0 ] && [ -f "${entitlements}" ] && command -v codesign >/dev/null 2>&1; then
	# Check if the command is a linker command.
	case "$1" in
	*link)
		shift
		# Find a parameter that is a output file.
		while [ $# -gt 1 ]; do
			case "$1" in
			-o)
				# If the output file is a executable, sign it with the entitlements file.
				if [ -x "$2" ]; then
					codesign_output="$(codesign -v --entitlements "${entitlements}" -s - "$2" 2>&1)"
					echo "$$: ${codesign_output}" >>"${logfile}"
				fi
				break
				;;
			*) shift ;;
			esac
		done
		;;
	*) ;;
	esac
fi

# Print the output of the command and exit with the result of the command.
echo "${output}"
exit "${result}"
