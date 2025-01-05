# See also: package.json
NPM ?= npm

build serve: _output/docsy
	$(NPM) run $@

_output/docsy:
	$(MAKE) -C .. docsy

clean:
	$(NPM) run $@
	rm -Rf _output
