---
title: Recording demos
weight: 40
---

Record real commands against disposable instances using the latest
[released Lima](https://github.com/lima-vm/lima/releases/latest). Check
`limactl --version` before recording; the development documentation may describe
features that are not in that release. Use a separate `LIMA_HOME` for capture
instances and mount only a disposable sample project when a demonstration needs
host files. Keep credentials, personal paths, and unrelated windows out of view.

Prepare images, packages, and containers before recording. For the landing demo,
stop the prepared default instance, record `limactl start default`, and finish
with `lima uname -a`. Keep actual startup output and the final result readable.
For use cases, aim for 5–10 seconds and show the operation succeeding, not just
the setup. Run Kubernetes commands on the host, show the restricted project mount
for an agent inside the VM, and show a guest tool result for an MCP demonstration.

[VHS](https://github.com/charmbracelet/vhs) can record terminal commands directly;
it requires `ttyd` and FFmpeg. Use a clean shell prompt, a readable monospace font,
and `Wait` for command completion instead of assuming a fixed startup time.
Capture GUI scenarios with a screen recorder. Trim installation and idle time,
but do not replace command output. Preserve enough context to identify the host,
guest, and demonstrated operation.

Encode silent, browser-compatible MP4 files with H.264, an even frame size,
`yuv420p` pixels, and the index at the start of the file. For example, from the
`website` directory, after trimming a containerd recording to `containerd-trimmed.mp4`:

```sh
ffmpeg -i containerd-trimmed.mp4 -map 0:v:0 -an -map_metadata -1 \
  -vf 'scale=trunc(iw/2)*2:trunc(ih/2)*2' \
  -c:v libx264 -crf 23 -preset slow -pix_fmt yuv420p -movflags +faststart \
  static/videos/usecases/containerd.mp4
```

Convert the trimmed landing recording to a palette-based GIF:

```sh
ffmpeg -i demo-trimmed.mp4 -filter_complex \
  '[0:v]fps=10,split[a][b];[a]palettegen[p];[b][p]paletteuse' \
  -loop 0 -map_metadata -1 static/images/demo.gif
```

Watch every encoded asset and compare its size and text legibility with the
source before accepting it. Check desktop and narrow layouts, keyboard playback
controls, direct media links, and loading behavior (`preload="none"`, no autoplay).
Run `make -C website build` from the repository root and check the rendered guide
links and media URLs; `npm test` currently does not audit links.

For each accepted recording, keep a capture record here with its asset filename,
capture date, exact Lima version, host OS and architecture, guest OS and template,
preparation commands, and recorded commands. Include the public run URL and job
name for GitHub Actions footage. Update the record whenever an asset is replaced.
