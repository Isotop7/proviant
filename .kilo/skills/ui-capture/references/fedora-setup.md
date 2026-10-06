# Fedora host notes

Everything below was established by running the tool on Fedora 44 (Xfce),
kernel 7.2.8-200.fc44, x86_64.

## Why the browser runs in a container

Playwright officially supports Debian/Ubuntu only. On Fedora
`npx playwright install-deps` fails because it shells out to `apt-get`
(playwright#27890), and installing Chromium plus its library set by hand is a
long, brittle list. The Playwright container image sidesteps all of it.

## The image must match the installed package

The image ships the **browsers** at `/ms-playwright` but **not** the `playwright`
npm package — which is why every official example runs `npm install` first.
`playwright-core` refuses to start a browser whose revision does not match its
own, so the tag is derived at runtime from the package the runner installed and
must equal that version exactly. The pin is `PLAYWRIGHT_VERSION` in
`scripts/uicapture.mjs` (exact, no caret range) for the same reason.

The runner installs it into `.kilo/skills/ui-capture/node_modules` with
`--ignore-scripts`, so no browser download happens — the image already has the
browsers and the host must not acquire a second copy.

One-time setup:

```bash
podman pull mcr.microsoft.com/playwright:v1.63.0-noble
```

`uicapture` never pulls implicitly — an unattended 2.5 GB download in the middle
of an agent run is not acceptable. It exits 9 and prints the exact pull command.

## The Go server stays on the host

The image has no Go toolchain, and a CGO binary built against Fedora 44's
glibc 2.41 will not run on the image's Ubuntu 24.04 (glibc 2.39). So the server
is built and run natively; the container reaches it through `--network=host`,
which shares the host loopback and needs no port publishing.

## The container runs unprivileged so Chromium can sandbox

Chromium refuses to start as root without `--no-sandbox`, and Playwright's
default (`chromiumSandbox: false`) takes that route — leaving an unsandboxed
renderer that loads user-supplied product image URLs. So the runner launches as
your user and lets Chromium use its own seccomp sandbox:

- podman: `--userns=keep-id`
- docker: `--user <uid>:<gid>`

Do **not** pass `--user` on rootless podman. Inside the user namespace a non-zero
container uid does not map to the same host uid — it falls through to `nobody`,
which cannot write your output directory:

```
touch .cache/ui-captures/<run>/probe.txt: Permission denied
```

`--userns=keep-id` maps your host uid in as itself instead.

`--cap-drop=ALL --cap-add=SYS_CHROOT` is the capability set: the Chromium zygote
needs `chroot` and nothing else. With `--security-opt=no-new-privileges` and
read-only mounts that leaves a renderer compromise without a write foothold in
your source tree.

## SELinux: mount narrowly

`:Z` relabels the mounted tree to `container_file_t`, which is fine for a
throwaway container but not something to inflict on a source tree. Mounting the
whole repo therefore relabels every tracked file — including `.git` — and the
label survives after the container exits.

`uicapture` mounts only `.kilo/` and the subtree holding the output directory.
Both are gitignored, so a run never touches tracked files' labels. Everything is
mounted read-only except the output directory.

If a previous tool or your own `task check` already relabelled the repo, restore
it as your normal user:

```bash
restorecon -R ~/dev/proviant
```

(`Taskfile.yml:62-64` does `-v ./:/app:Z` from the repo root, so `task check`
relabels the whole tree too. That is a separate, pre-existing habit.)

## ffmpeg on Fedora has no libx264

The image's bundled ffmpeg is a stripped VP8 build with no H.264 encoder at all,
so transcoding has to happen on the host. Fedora ships `ffmpeg-free`, which also
has no `libx264`:

```
$ ffmpeg -hide_banner -y -i in.webm -c:v libx264 out.mp4
Unknown encoder 'libx264'
```

`ffmpeg-free` does provide `libopenh264`, so the runner probes
`libx264 → libopenh264 → h264_vaapi` and uses the first one present, saying so
on stderr. For libx264, install the full build:

```bash
sudo dnf install ffmpeg-libs   # replaces ffmpeg-free with libx264
```

gif transcoding needs no codec beyond `palettegen`/`paletteuse`, both present in
`ffmpeg-free`.

## `--browser local` on Fedora

Works, but needs the browsers and their libraries installed by hand:

```bash
npx playwright install chromium
sudo dnf install nss atk at-spi2-atk cups-libs libdrm libXcomposite \
  libXdamage libXfixes libXrandr mesa-libgbm alsa-lib pango cairo
```

Until then, local mode exits 4 with the missing-executable path. Container mode
is the supported path on this host.

## What this host does not have

- **tesseract** — `proviant.go:361` degrades OCR gracefully, and the runner
  pins `PROVIANT_OCR_ENABLED=false` anyway.
- **A valid SMTP/ntfy/telegram block** — `proviant.go:342` *panics* on an
  invalid notification config, so the runner pins
  `PROVIANT_NOTIFICATION_ENABLED=false`.
- **docker** — podman only. The runner mirrors `Taskfile.yml:5`
  (`podman || docker`) and exits 8 if neither exists.