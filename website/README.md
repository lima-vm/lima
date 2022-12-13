# The source of the Lima website (https://lima-vm.io)

This directory is the [Netlify base directory](https://docs.netlify.com/configure-builds/overview/) of [https://lima-vm.io](https://lima-vm.io/) .

The actual contents are generated from the markdown files on the browser side:
- [`../README.md`](../README.md)
- [`../docs/*.md`](../docs/)
- [`../examples/README.md`](../examples/README.md)

The site is previewable and deployable with just the single [`index.html`](./index.html).

No dependency on any templating engine currently, but eventually we may adopt docsy or something else similar.
