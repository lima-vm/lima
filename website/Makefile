# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0
# See also: package.json
NPM ?= npm

build serve: _output/docsy
	$(NPM) run $@

_output/docsy:
	$(MAKE) -C .. docsy

clean:
	$(NPM) run $@
	rm -Rf _output
